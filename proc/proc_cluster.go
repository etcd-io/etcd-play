// Copyright 2016 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package proc

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"path"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coreos/etcd/clientv3"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/tools/functional-tester/etcd-agent/client"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

var (
	colorsToHTML = []string{
		"#ff0000", // red
		"#008000", // green
		"#ff9933", // yellow
		"#0000ff", // blue
		"#ff00ff", // magenta
	}
)

// Node contains node operations.
type Node interface {
	// Endpoint returns the gRPC endpoint.
	Endpoint() string

	// StatusEndpoint returns the v2 status endpoint.
	StatusEndpoint() string

	// IsActive returns true if the Node is running(active).
	IsActive() bool

	// Start starts Node process.
	Start() error

	// Restart restarts Node process.
	Restart() error

	// Terminate kills the Node process.
	Terminate() error

	// Clean cleans up the resources from the Node. This must be called
	// after Terminate.
	Clean() error

	// TLS returns the *tls.Config of the Node.
	TLS() *tls.Config
}

// ServerStatus encapsulates various statistics about an EtcdServer.
type ServerStatus struct {
	Name         string
	ID           string
	Endpoint     string
	State        string
	NumberOfKeys int
	Hash         int
}

// Cluster controls a set of Nodes.
type Cluster interface {
	// Write writes messages to a Node process.
	Write(name, msg string, streamIDs ...string) error

	// SharedStream returns a shared stream.
	SharedStream() chan string

	// Stream returns the channel for streaming logs.
	Stream(streamID string) chan string

	// Start starts Node process.
	Start(name string) error

	// Restart restarts Node process.
	Restart(name string) error

	// Revive restarts all Nodes in case no Node is up for a certain period of
	// time.
	Revive() error

	// Terminate kills the Node process.
	Terminate(name string) error

	// Clean cleans up the resources from the Node. This must be called
	// after Terminate.
	Clean(name string) error

	// Bootstrap starts all Node processes.
	Bootstrap() error

	// Shutdown terminates and cleans all Nodes.
	Shutdown() error

	// Endpoints returns all endpoints for clients and a map of name and endpoint, vice versa.
	Endpoints() ([]string, map[string]string, map[string]string)

	// Leader returns the name of the leader.
	Leader() (string, error)

	// Status returns all endpoints and status of the cluster.
	Status() (map[string]ServerStatus, error)

	// Put puts key-value to the cluster. If the name is not specified, it
	// sends request to a random node.
	Put(name, key, value string, streamIDs ...string) error

	// Get get the value from the key. If the name is not specified,
	// it gets from a random node.
	Get(name, key string, streamIDs ...string) ([]string, error)

	// Delete deletes the key.
	Delete(ame, key string, streamIDs ...string) error

	// Stress stresses the cluster. If the name is not specified, it stresses
	// random nodes.
	Stress(name string, stressN int, streamIDs ...string) error
}

// defaultCluster groups a set of Node processes.
type defaultCluster struct {
	mu           sync.Mutex // guards the following
	sharedStream chan string
	idToStream   map[string]chan string
	nameToNode   map[string]Node
	epToName     map[string]string
}

type NodeType int

const (
	WebLocal NodeType = iota
	WebRemote
)

type op struct {
	liveLog        bool
	limitInterval  time.Duration
	agentEndpoints []string
}

func (o *op) apply(opts []OpOption) {
	for _, opt := range opts {
		opt(o)
	}
}

type OpOption func(*op)

// WithLiveLog feeds etcd logs real-time. Only applicable for
// 'etcd-play web' command in localhost.
func WithLiveLog() OpOption {
	return func(o *op) {
		o.liveLog = true
	}
}

// WithLimitInterval puts limit interval between terminate and immediate restart,
// restart and immediate terminate.
func WithLimitInterval(d time.Duration) OpOption {
	return func(o *op) {
		o.limitInterval = d
	}
}

// WithAgentEndpoins specifies etcd-agent endpoints. Only applicable for
// 'etcd-play web' command when deployed with remote machines.
func WithAgentEndpoints(eps []string) OpOption {
	return func(o *op) {
		o.agentEndpoints = eps
	}
}

// NewCluster creates Cluster with generated flags.
func NewCluster(opt NodeType, programPath string, fs []*Flags, opts ...OpOption) (Cluster, error) {
	if len(fs) == 0 {
		return nil, nil
	}

	o := &op{}
	o.apply(opts)

	if len(o.agentEndpoints) > 0 && opt == WebRemote {
		if len(o.agentEndpoints) != len(fs) {
			return nil, fmt.Errorf("agent endpoints must be the same size of flags (%d != %d)", len(o.agentEndpoints), len(fs))
		}
	}

	if err := CombineFlags(opt == WebRemote, fs...); err != nil {
		return nil, err
	}

	bufferedStream := make(chan string, 5000)
	c := &defaultCluster{
		mu:           sync.Mutex{},
		sharedStream: bufferedStream,
		idToStream:   make(map[string]chan string),
		nameToNode:   make(map[string]Node),
		epToName:     make(map[string]string),
	}

	var maxProcNameLength, colorIdx int
	for i, f := range fs {
		if colorIdx >= len(colorsToHTML) {
			colorIdx = 0
		}

		name := f.Name
		if len(name) > maxProcNameLength {
			maxProcNameLength = len(name)
		}

		var (
			certPath = path.Join(f.DataDir, "fixtures/client/cert.pem")
			keyPath  = path.Join(f.DataDir, "fixtures/client/key.pem")
		)
		var ni Node
		switch opt {
		case WebLocal:
			ni = &NodeWebLocal{
				pmu:                &c.mu,
				pmaxProcNameLength: &maxProcNameLength,
				colorIdx:           colorIdx,
				liveLog:            o.liveLog,
				sharedStream:       bufferedStream, // shared by all nodes
				ProgramPath:        programPath,
				Flags:              f,
				TLSCertPath:        certPath,
				TLSKeyPath:         keyPath,
				TLSConfig:          nil,
				cmd:                nil,
				PID:                0,
				active:             false,
				limitInterval:      o.limitInterval,
			}

		case WebRemote:
			if len(o.agentEndpoints) == 0 {
				return nil, fmt.Errorf("no agent endpoints found")
			}
			a, err := client.NewAgent(o.agentEndpoints[i])
			if err != nil {
				return nil, err
			}
			ni = &NodeWebRemoteClient{
				Flags:         f,
				TLSCertPath:   certPath,
				TLSKeyPath:    keyPath,
				TLSConfig:     nil,
				Agent:         a,
				active:        false,
				limitInterval: o.limitInterval,
			}

		default:
			return nil, fmt.Errorf("NodeType %v is not implemented", opt)
		}
		c.nameToNode[name] = ni

		colorIdx++
	}

	return c, nil
}

func (c *defaultCluster) Write(name, msg string, streamIDs ...string) error {
	c.mu.Lock()
	nd, ok := c.nameToNode[name]
	c.mu.Unlock()

	if !ok {
		return fmt.Errorf("%s does not exist", name)
	}

	switch vt := nd.(type) {
	case *NodeWebLocal:
		if len(streamIDs) == 0 {
			vt.sharedStream <- msg
		} else {
			for _, streamID := range streamIDs {
				c.Stream(streamID) <- msg
			}
		}

	case *NodeWebRemoteClient:
		if len(streamIDs) > 0 {
			for _, streamID := range streamIDs {
				c.Stream(streamID) <- msg
			}
		}

	default:
		return fmt.Errorf("%v does not implement Write", reflect.TypeOf(nd))
	}
	return nil
}

func (c *defaultCluster) SharedStream() chan string {
	if c == nil {
		return nil
	}
	return c.sharedStream
}

func (c *defaultCluster) Stream(streamID string) chan string {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.idToStream[streamID]
	if ok {
		return v
	}
	ch := make(chan string, 5000)
	c.idToStream[streamID] = ch
	return ch
}

func (c *defaultCluster) Start(name string) error {
	c.mu.Lock()
	nd, ok := c.nameToNode[name]
	c.mu.Unlock()
	if !ok {
		return fmt.Errorf("%s does not exist", name)
	}
	return nd.Start()
}

func (c *defaultCluster) Restart(name string) error {
	c.mu.Lock()
	nd, ok := c.nameToNode[name]
	c.mu.Unlock()
	if !ok {
		return fmt.Errorf("%s does not exist", name)
	}
	return nd.Restart()
}

func (c *defaultCluster) Revive() error {
	for _, nd := range c.nameToNode {
		if nd.IsActive() {
			return nil
		}
	}
	for _, nd := range c.nameToNode {
		if err := nd.Restart(); err != nil {
			return err
		}
	}
	return nil
}

func (c *defaultCluster) Terminate(name string) error {
	c.mu.Lock()
	nd, ok := c.nameToNode[name]
	c.mu.Unlock()
	if !ok {
		return fmt.Errorf("%s does not exist", name)
	}
	return nd.Terminate()
}

func (c *defaultCluster) Clean(name string) error {
	c.mu.Lock()
	nd, ok := c.nameToNode[name]
	c.mu.Unlock()
	if !ok {
		return fmt.Errorf("%s does not exist", name)
	}
	return nd.Clean()
}

func (c *defaultCluster) Bootstrap() error {
	if len(c.nameToNode) == 0 {
		return nil
	}
	done, errC := make(chan struct{}), make(chan error)
	for name, nd := range c.nameToNode {
		go func(name string, nd Node) {
			fmt.Println("Starting", name)
			err := nd.Start()
			if err != nil {
				errC <- fmt.Errorf("%s (%v)", name, err)
				return
			}
			done <- struct{}{}
		}(name, nd)
	}

	cn := 0
	for cn != len(c.nameToNode) {
		select {
		case <-done:
		case err := <-errC:
			return err
		}
		cn++
	}

	sc := make(chan os.Signal, 10)
	signal.Notify(sc, os.Interrupt, os.Kill)
	s := <-sc
	log.Printf("Got signal %s... shutting down...", s)
	return c.Shutdown()
}

func (c *defaultCluster) Shutdown() error {
	if len(c.nameToNode) == 0 {
		return nil
	}
	var wg sync.WaitGroup
	wg.Add(len(c.nameToNode))
	for name, nd := range c.nameToNode {
		go func(name string, nd Node) {
			defer wg.Done()
			if err := nd.Terminate(); err != nil {
				log.Printf("Terminate(%s): error (%v)", name, err)
			}
			if err := nd.Clean(); err != nil {
				log.Printf("Clean(%s): error (%v)", name, err)
			}
		}(name, nd)
	}
	wg.Wait()
	return nil
}

func (c *defaultCluster) Endpoints() ([]string, map[string]string, map[string]string) {
	var (
		endpoints          []string
		nameToGRPCEndpoint = make(map[string]string)
		grpcEndpointToName = make(map[string]string)
	)
	for n, nd := range c.nameToNode {
		if nd.Endpoint() != "" && nd.IsActive() {
			endpoints = append(endpoints, nd.Endpoint())
		}
		ep := nd.Endpoint()
		nameToGRPCEndpoint[n] = ep
		grpcEndpointToName[ep] = n
	}
	sort.Strings(endpoints)
	return endpoints, nameToGRPCEndpoint, grpcEndpointToName
}

func (c *defaultCluster) Leader() (string, error) {
	endpoints, _, epToName := c.Endpoints()
	var lerr error
	for _, ep := range endpoints {
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   []string{ep},
			DialTimeout: 5 * time.Second,
		})
		if err != nil {
			lerr = err
			continue
		}
		defer cli.Close()

		mapi := clientv3.NewMaintenance(cli)
		resp, err := mapi.Status(context.Background(), ep)
		if err != nil {
			lerr = err
			continue
		}

		if resp.Header.MemberId == resp.Leader {
			return epToName[ep], nil
		}
		lerr = nil
	}
	return "", fmt.Errorf("no leader found (%v)", lerr)
}

var emptyStat = ServerStatus{
	Name:         "",
	ID:           "unknown",
	Endpoint:     "unknown",
	State:        "unreachable",
	NumberOfKeys: 0,
	Hash:         0,
}

func getStatus(name, grpcEndpoint, v2Endpoint string, rs chan ServerStatus, errc chan error) {
	// func getStatus(name, grpcEndpoint, v2Endpoint string, tlsConfig *tls.Config, rs chan ServerStatus, errc chan error) {
	// tc := credentials.NewTLS(tlsConfig)
	// conn, err := grpc.Dial(grpcEndpoint, grpc.WithTransportCredentials(tc), grpc.WithTimeout(5*time.Second))

	conn, err := grpc.Dial(grpcEndpoint, grpc.WithInsecure(), grpc.WithTimeout(5*time.Second))
	if err != nil {
		errc <- err
		return
	}
	defer conn.Close()

	stat := emptyStat
	stat.Name = name
	stat.Endpoint = grpcEndpoint

	done, errChan := make(chan struct{}), make(chan error)

	// ID, State
	go func() {
		mapi := pb.NewMaintenanceClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		sts, err := mapi.Status(ctx, &pb.StatusRequest{})
		cancel()
		if err != nil {
			errChan <- err
			return
		}
		mid := sts.Header.MemberId
		stat.ID = fmt.Sprintf("%x", mid)
		stat.State = "Follower"
		if mid == sts.Leader {
			stat.State = "Leader"
		}
		done <- struct{}{}
	}()
	select {
	case <-time.After(5 * time.Second):
		errc <- fmt.Errorf("timed out")
		return
	case err := <-errChan:
		errc <- err
		return
	case <-done:
	}

	// Hash
	go func() {
		mc := pb.NewMaintenanceClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		resp, err := mc.Hash(ctx, &pb.HashRequest{})
		cancel()
		if err != nil {
			errChan <- err
			return
		}
		stat.Hash = int(resp.Hash)
		done <- struct{}{}
	}()
	select {
	case <-time.After(5 * time.Second):
		errc <- fmt.Errorf("timed out")
		return
	case err := <-errChan:
		errc <- err
		return
	case <-done:
	}

	// Number of keys
	go func() {
		resp, err := http.Get(v2Endpoint + "/metrics")
		if err != nil {
			errChan <- err
			return
		}
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			txt := scanner.Text()
			if strings.HasPrefix(txt, "#") {
				continue
			}
			ts := strings.SplitN(txt, " ", 2)
			fv := 0.0
			if len(ts) == 2 {
				v, err := strconv.ParseFloat(ts[1], 64)
				if err == nil {
					fv = v
				}
			}
			if ts[0] == "etcd_debugging_mvcc_keys_total" {
				stat.NumberOfKeys = int(fv)
				break
			}
		}
		gracefulClose(resp)
		done <- struct{}{}
	}()
	select {
	case <-time.After(5 * time.Second):
		errc <- fmt.Errorf("timed out")
		return
	case err := <-errChan:
		errc <- err
		return
	case <-done:
		rs <- stat
	}
	return
}

func (c *defaultCluster) Status() (map[string]ServerStatus, error) {
	_, nameToEndpoint, _ := c.Endpoints()
	nameToV2Endpoint := make(map[string]string)
	for name, nd := range c.nameToNode {
		nameToV2Endpoint[name] = nd.StatusEndpoint()
	}

	sc, errc := make(chan ServerStatus), make(chan error)
	for name, grpcEndpoint := range nameToEndpoint {
		go getStatus(name, grpcEndpoint, nameToV2Endpoint[name], sc, errc)
		// go getStatus(name, grpcEndpoint, nameToV2Endpoint[name], c.nameToNode[name].TLS(), sc, errc)
	}

	nameToStatus := make(map[string]ServerStatus)
	var err error
	cn := 0
	for cn != len(nameToEndpoint) {
		select {
		case s := <-sc:
			nameToStatus[s.Name] = s
		case err = <-errc:
		}
		cn++
	}

	for name, endpoint := range nameToEndpoint {
		if _, ok := nameToStatus[name]; !ok {
			stat := emptyStat
			stat.Name = name
			stat.Endpoint = endpoint
			nameToStatus[name] = stat
		}
	}
	return nameToStatus, err
}

func (c *defaultCluster) Put(name, key, value string, streamIDs ...string) error {
	endpoints, nameToEndpoint, _ := c.Endpoints()
	if name == "" {
		for n := range nameToEndpoint {
			name = n
			break
		}
	}
	if v, ok := nameToEndpoint[name]; ok {
		endpoints = []string{v}
	} else {
		return fmt.Errorf("%s does not exist", name)
	}

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return err
	}
	defer cli.Close()

	kvc := clientv3.NewKV(cli)
	st := time.Now()

	c.Write(name, fmt.Sprintf("[PUT] Started! (endpoints: %q)", endpoints), streamIDs...)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	_, err = kvc.Put(ctx, key, value)
	cancel()
	if err != nil {
		return err
	}
	c.Write(name, fmt.Sprintf("[PUT] %q : %q / Took %v (endpoints: %q)", key, value, time.Since(st), endpoints), streamIDs...)

	return nil
}

func (c *defaultCluster) Get(name, key string, streamIDs ...string) ([]string, error) {
	endpoints, nameToEndpoint, _ := c.Endpoints()
	if name == "" {
		for n := range nameToEndpoint {
			name = n
			break
		}
	}
	if v, ok := nameToEndpoint[name]; ok {
		endpoints = []string{v}
	} else {
		return nil, fmt.Errorf("%s does not exist", name)
	}

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, err
	}
	defer cli.Close()

	kvc := clientv3.NewKV(cli)
	st := time.Now()

	c.Write(name, fmt.Sprintf("[GET] Started! (endpoints: %q)", endpoints), streamIDs...)

	var opts []clientv3.OpOption
	if len(key) == 0 {
		key = "\x00" // query the whole key
		opts = append(opts, clientv3.WithFromKey())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	resp, err := kvc.Get(ctx, key, opts...)
	cancel()
	if err != nil {
		return nil, err
	}
	vs := []string{}
	if len(resp.Kvs) > 0 {
		for _, ev := range resp.Kvs {
			vs = append(vs, string(ev.Value))
			c.Write(name, fmt.Sprintf("[GET] %q : %q", ev.Key, ev.Value), streamIDs...)
		}
	} else {
		c.Write(name, fmt.Sprintf("[GET] %q does not exist!", key), streamIDs...)
	}

	c.Write(name, fmt.Sprintf("[GET] Done! Took %v (endpoints: %q)", time.Since(st), endpoints), streamIDs...)
	sort.Strings(vs)
	return vs, nil
}

func (c *defaultCluster) Delete(name, key string, streamIDs ...string) error {
	endpoints, nameToEndpoint, _ := c.Endpoints()
	if name == "" {
		for n := range nameToEndpoint {
			name = n
			break
		}
	}
	if v, ok := nameToEndpoint[name]; ok {
		endpoints = []string{v}
	} else {
		return fmt.Errorf("%s does not exist", name)
	}

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return err
	}
	defer cli.Close()

	kvc := clientv3.NewKV(cli)
	st := time.Now()

	c.Write(name, fmt.Sprintf("[DELETE] Started! (endpoints: %q)", endpoints), streamIDs...)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	_, err = kvc.Delete(ctx, key)
	cancel()
	if err != nil {
		return err
	}
	c.Write(name, fmt.Sprintf("[DELETE] Done! Took %v (endpoints: %q)", time.Since(st), endpoints), streamIDs...)

	return nil
}

func (c *defaultCluster) stress(name string, stressN int, donec chan struct{}, errc chan error, streamIDs ...string) {
	endpoints, nameToEndpoint, _ := c.Endpoints()
	if name == "" {
		for n := range nameToEndpoint {
			name = n
			break
		}
	}
	if v, ok := nameToEndpoint[name]; ok {
		endpoints = []string{v}
	}
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		errc <- err
		return
	}
	defer cli.Close()

	clientsN := 10 // 1 connection, 10 clients
	kvcs := make([]clientv3.KV, clientsN)
	for i := range kvcs {
		kvcs[i] = clientv3.NewKV(cli)
	}

	keys, vals := multiRandBytes(5, stressN), multiRandBytes(5, stressN)
	st := time.Now()
	done, errChan := make(chan struct{}), make(chan error)
	for i := 0; i < stressN; i++ {
		go func(i int) {
			kvc := kvcs[rand.Intn(clientsN)]
			key, val := fmt.Sprintf("sample_%d_%s", i, keys[i]), string(vals[i])
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			_, err = kvc.Put(ctx, key, val)
			cancel()
			if err != nil {
				errChan <- err
				return
			}
			c.Write(name, fmt.Sprintf("[STRESS PUT %2d] %q : %q", i, key, val), streamIDs...)
			done <- struct{}{}
		}(i)
	}
	cn := 0
	for cn != stressN {
		select {
		case err := <-errChan:
			errc <- err
			return
		case <-done:
			cn++
		}
	}
	tt := time.Since(st)
	pt := tt / time.Duration(stressN)

	c.Write(name, fmt.Sprintf("[STRESS] Done! Took %v for %d requests(%v per each), %d client(s) (endpoints: %s)", tt, stressN, pt, clientsN, endpoints), streamIDs...)
	donec <- struct{}{}
	return
}

func (c *defaultCluster) Stress(name string, stressN int, streamIDs ...string) error {
	donec, errc := make(chan struct{}), make(chan error)
	go c.stress(name, stressN, donec, errc, streamIDs...)
	select {
	case err := <-errc:
		return err
	case <-donec:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("Stress timed out!")
	}
}
