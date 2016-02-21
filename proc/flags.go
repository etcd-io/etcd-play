package proc

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/gyuho/psn/ss"
	"github.com/satori/go.uuid"
)

// Flags is a set of etcd flags.
type Flags struct {
	// Name is a name for an etcd node.
	Name string `flag:"name"`

	// ExperimentalV3Demo is either 'true' or 'false'.
	ExperimentalV3Demo bool `flag:"experimental-v3demo"`

	// ExperimentalgRPCAddr is used as an endpoint for gRPC.
	// It is usually composed of host, and port '*78'.
	// Default values are '127.0.0.1:2378'.
	ExperimentalgRPCAddr string `flag:"experimental-gRPC-addr"`

	// ListenClientURLs is a list of URLs to listen for clients.
	// It is usually composed of scheme, host, and port '*79'.
	// Default values are
	// 'http://localhost:2379,http://localhost:4001'.
	ListenClientURLs map[string]struct{} `flag:"listen-client-urls"`

	// AdvertiseClientURLs is a list of this node's client URLs
	// to advertise to the public. The client URLs advertised should
	// be accessible to machines that talk to etcd cluster. etcd client
	// libraries parse these URLs to connect to the cluster.
	// It is usually composed of scheme, host, and port '*79'.
	// Default values are
	// 'http://localhost:2379,http://localhost:4001'.
	AdvertiseClientURLs map[string]struct{} `flag:"advertise-client-urls"`

	// ListenPeerURLs is a list of URLs to listen on for peer traffic.
	// It is usually composed of scheme, host, and port '*80'.
	// Default values are
	// 'http://localhost:2380,http://localhost:7001'.
	ListenPeerURLs map[string]struct{} `flag:"listen-peer-urls"`

	// InitialAdvertisePeerURLs is URL to advertise to other nodes
	// in the cluster, used to communicate between nodes.
	// It is usually composed of scheme, host, and port '*80'.
	// Default values are
	// 'http://localhost:2380,http://localhost:7001'.
	InitialAdvertisePeerURLs map[string]struct{} `flag:"initial-advertise-peer-urls"`

	// InitialCluster is a map of each node name to its
	// InitialAdvertisePeerURLs.
	InitialCluster map[string]string `flag:"initial-cluster"`

	// InitialClusterToken is a token specific to cluster.
	// Specifying this can protect you from unintended cross-cluster
	// interaction when running multiple clusters.
	InitialClusterToken string `flag:"initial-cluster-token"`

	// InitialClusterState is either 'new' or 'existing'.
	InitialClusterState string `flag:"initial-cluster-state"`

	// EnablePprof is either 'true' or 'false'.
	EnablePprof bool `flag:"enable-pprof"`

	// DataDir is a directory to store its database.
	// It should be suffixed with '.etcd'.
	DataDir string `flag:"data-dir"`

	// Proxy is either 'on' or 'off'.
	Proxy bool `flag:"proxy"`

	ClientCertFile      string `flag:"cert-file"`        // Path to the client server TLS cert file.
	ClientKeyFile       string `flag:"key-file"`         // Path to the client server TLS key file.
	ClientCertAuth      bool   `flag:"client-cert-auth"` // Enable client cert authentication.
	ClientTrustedCAFile string `flag:"trusted-ca-file"`  // Path to the client server TLS trusted CA key file.

	PeerCertFile       string `flag:"peer-cert-file"`        // Path to the peer server TLS cert file.
	PeerKeyFile        string `flag:"peer-key-file"`         // Path to the peer server TLS key file.
	PeerClientCertAuth bool   `flag:"peer-client-cert-auth"` // Enable peer client cert authentication.
	PeerTrustedCAFile  string `flag:"peer-trusted-ca-file"`  // Path to the peer server TLS trusted CA file.
}

func defaultFlags() *Flags {
	fs := &Flags{}
	fs.ExperimentalV3Demo = true
	fs.ExperimentalgRPCAddr = "localhost:2378"
	fs.ListenClientURLs = map[string]struct{}{"http://localhost:2379": struct{}{}}
	fs.AdvertiseClientURLs = map[string]struct{}{"http://localhost:2379": struct{}{}}
	fs.ListenPeerURLs = map[string]struct{}{"http://localhost:2380": struct{}{}}
	fs.InitialAdvertisePeerURLs = map[string]struct{}{"http://localhost:2380": struct{}{}}
	fs.InitialCluster = make(map[string]string)
	fs.InitialClusterState = "new"
	fs.EnablePprof = true
	fs.DataDir = uuid.NewV4().String() + ".etcd"
	return fs
}

var globalPortPrefix uint32 = 12

// GenerateFlags returns generated default flags.
func GenerateFlags(name, host string, remote bool, usedPorts *ss.Ports) (*Flags, error) {
	// To allow ports between 1178 ~ 65480.
	// Therefore, prefix must be between 11 and 654.
	//
	// portPrefix < 11 || portPrefix > 654
	portPrefix := atomic.LoadUint32(&globalPortPrefix)
	if !remote {
		atomic.AddUint32(&globalPortPrefix, 1)
	}

	gRPCPort := fmt.Sprintf(":%d78", portPrefix)
	clientURLPort := fmt.Sprintf(":%d79", portPrefix)
	peerURLPort := fmt.Sprintf(":%d80", portPrefix)
	if !remote && usedPorts != nil {
		pts, err := ss.GetFreePorts(3, ss.TCP, ss.TCP6)
		if err != nil {
			return nil, err
		}
		if usedPorts.Exist(gRPCPort) {
			gRPCPort = pts[0]
		}
		if usedPorts.Exist(clientURLPort) {
			clientURLPort = pts[1]
		}
		if usedPorts.Exist(peerURLPort) {
			peerURLPort = pts[2]
		}
	}

	hs := "localhost"
	if host != "" {
		hs = host
	}
	gRPCAddr := hs + gRPCPort
	clientURL := "http://" + hs + clientURLPort
	peerURL := "http://" + hs + peerURLPort

	// TODO: automatic TLS
	//
	// if isClientTLS {
	// 	clientURL = strings.Replace(clientURL, "http://", "https://", -1)
	// }
	// if isPeerTLS {
	// 	peerURL = strings.Replace(peerURL, "http://", "https://", -1)
	// }

	fs := defaultFlags()
	fs.Name = name
	fs.ExperimentalgRPCAddr = gRPCAddr

	fs.ListenClientURLs = map[string]struct{}{clientURL: struct{}{}}
	fs.AdvertiseClientURLs = map[string]struct{}{clientURL: struct{}{}}

	fs.ListenPeerURLs = map[string]struct{}{peerURL: struct{}{}}
	fs.InitialAdvertisePeerURLs = map[string]struct{}{peerURL: struct{}{}}

	return fs, nil
}

// CombineFlags combine flags under a same cluster.
func CombineFlags(remote bool, cs ...*Flags) error {
	nameToPeerURL := make(map[string]string)
	portCheck := ""
	for i := range cs {
		if _, ok := nameToPeerURL[cs[i].Name]; ok {
			return fmt.Errorf("%s is duplicate!", cs[i].Name)
		}
		tp := strings.Join(cs[i].getAllPorts(), "___")
		if portCheck == "" {
			portCheck = tp
		} else if portCheck == tp && !remote {
			return fmt.Errorf("%q has duplicate ports in another node!", cs[i].getAllPorts())
		}
		nameToPeerURL[cs[i].Name] = mapToCommaString(cs[i].InitialAdvertisePeerURLs)
	}
	token := uuid.NewV4().String() // must be same (used for cluster-id)
	for i := range cs {
		cs[i].InitialClusterToken = token
		cs[i].InitialCluster = nameToPeerURL
	}
	return nil
}

func (f *Flags) IsValid() (bool, error) {
	if len(f.Name) == 0 {
		return false, errors.New("Name must be specified!")
	}
	if f.InitialClusterState != "new" && f.InitialClusterState != "existing" {
		return false, errors.New("InitialClusterState must be either 'new' or 'existing'.")
	}
	return true, nil
}

func (f *Flags) Pairs() ([][]string, error) {
	valid, e := f.IsValid()
	if !valid || e != nil {
		return nil, e
	}

	var pairs [][]string

	nameTag, err := f.getTag("Name")
	if err != nil {
		return nil, err
	}
	pairs = append(pairs, []string{nameTag, strings.TrimSpace(f.Name)})

	experimentV3DemoTag, err := f.getTag("ExperimentalV3Demo")
	if err != nil {
		return nil, err
	}
	if f.ExperimentalV3Demo {
		pairs = append(pairs, []string{experimentV3DemoTag, "true"})
	}

	experimentalgRPCAddrTag, err := f.getTag("ExperimentalgRPCAddr")
	if err != nil {
		return nil, err
	}
	pairs = append(pairs, []string{experimentalgRPCAddrTag, strings.TrimSpace(f.ExperimentalgRPCAddr)})

	listenClientURLsTag, err := f.getTag("ListenClientURLs")
	if err != nil {
		return nil, err
	}
	pairs = append(pairs, []string{listenClientURLsTag, mapToCommaString(f.ListenClientURLs)})

	advertiseClientURLsTag, err := f.getTag("AdvertiseClientURLs")
	if err != nil {
		return nil, err
	}
	pairs = append(pairs, []string{advertiseClientURLsTag, mapToCommaString(f.AdvertiseClientURLs)})

	listenPeerURLsTag, err := f.getTag("ListenPeerURLs")
	if err != nil {
		return nil, err
	}
	pairs = append(pairs, []string{listenPeerURLsTag, mapToCommaString(f.ListenPeerURLs)})

	initialAdvertisePeerURLsTag, err := f.getTag("InitialAdvertisePeerURLs")
	if err != nil {
		return nil, err
	}
	pairs = append(pairs, []string{initialAdvertisePeerURLsTag, mapToCommaString(f.InitialAdvertisePeerURLs)})

	initialClusterTag, err := f.getTag("InitialCluster")
	if err != nil {
		return nil, err
	}
	pairs = append(pairs, []string{initialClusterTag, mapToMapString(f.InitialCluster)})

	initialClusterTokenTag, err := f.getTag("InitialClusterToken")
	if err != nil {
		return nil, err
	}
	pairs = append(pairs, []string{initialClusterTokenTag, f.InitialClusterToken})

	initialClusterStateTag, err := f.getTag("InitialClusterState")
	if err != nil {
		return nil, err
	}
	if f.InitialClusterState == "new" {
		pairs = append(pairs, []string{initialClusterStateTag, "new"})
	} else {
		pairs = append(pairs, []string{initialClusterStateTag, "existing"})
	}

	enablePprofTag, err := f.getTag("EnablePprof")
	if err != nil {
		return nil, err
	}
	if f.EnablePprof {
		pairs = append(pairs, []string{enablePprofTag, "true"})
	}

	dataDirTag, err := f.getTag("DataDir")
	if err != nil {
		return nil, err
	}
	pairs = append(pairs, []string{dataDirTag, strings.TrimSpace(f.DataDir)})

	proxyTag, err := f.getTag("Proxy")
	if err != nil {
		return nil, err
	}
	if f.Proxy {
		pairs = append(pairs, []string{proxyTag, "on"})
	}

	if f.ClientCertFile != "" && f.ClientKeyFile != "" {
		clientCertTag, err := f.getTag("ClientCertFile")
		if err != nil {
			return nil, err
		}
		pairs = append(pairs, []string{clientCertTag, f.ClientCertFile})

		clientKeyTag, err := f.getTag("ClientKeyFile")
		if err != nil {
			return nil, err
		}
		pairs = append(pairs, []string{clientKeyTag, f.ClientKeyFile})

		clientClientCertAuthTag, err := f.getTag("ClientCertAuth")
		if err != nil {
			return nil, err
		}
		pairs = append(pairs, []string{clientClientCertAuthTag, "true"})

		clientTrustedCAFileTag, err := f.getTag("ClientTrustedCAFile")
		if err != nil {
			return nil, err
		}
		pairs = append(pairs, []string{clientTrustedCAFileTag, f.PeerTrustedCAFile})
	}

	if f.PeerCertFile != "" && f.PeerKeyFile != "" {
		peerCertTag, err := f.getTag("PeerCertFile")
		if err != nil {
			return nil, err
		}
		pairs = append(pairs, []string{peerCertTag, f.PeerCertFile})

		peerKeyTag, err := f.getTag("PeerKeyFile")
		if err != nil {
			return nil, err
		}
		pairs = append(pairs, []string{peerKeyTag, f.PeerKeyFile})

		peerClientCertAuthTag, err := f.getTag("PeerClientCertAuth")
		if err != nil {
			return nil, err
		}
		pairs = append(pairs, []string{peerClientCertAuthTag, "true"})

		peerTrustedCAFileTag, err := f.getTag("PeerTrustedCAFile")
		if err != nil {
			return nil, err
		}
		pairs = append(pairs, []string{peerTrustedCAFileTag, f.PeerTrustedCAFile})
	}

	return pairs, nil
}

func (f *Flags) StringSlice() ([]string, error) {
	pairs, err := f.Pairs()
	if err != nil {
		return nil, err
	}
	slice := []string{}
	for _, pair := range pairs {
		if pair[1] == "true" {
			slice = append(slice, "--"+pair[0])
		} else {
			slice = append(slice, "--"+pair[0], pair[1])
		}
	}
	return slice, nil
}

func (f *Flags) String() (string, error) {
	pairs, err := f.Pairs()
	if err != nil {
		return "", err
	}
	sb := new(bytes.Buffer)
	for _, pair := range pairs {
		sb.WriteString(fmt.Sprintf("--%s='%s'", pair[0], pair[1]))
		sb.WriteString(" ")
	}
	return strings.TrimSpace(sb.String()), nil
}

func (f *Flags) getAllPorts() []string {
	tm := make(map[string]struct{})
	if f.ExperimentalgRPCAddr != "" {
		ss := strings.Split(f.ExperimentalgRPCAddr, ":")
		tm[strings.TrimSpace(ss[len(ss)-1])] = struct{}{}
	}
	for k := range f.ListenClientURLs {
		ss := strings.Split(k, ":")
		tm[strings.TrimSpace(ss[len(ss)-1])] = struct{}{}
	}
	for k := range f.AdvertiseClientURLs {
		ss := strings.Split(k, ":")
		tm[strings.TrimSpace(ss[len(ss)-1])] = struct{}{}
	}
	for k := range f.ListenPeerURLs {
		ss := strings.Split(k, ":")
		tm[strings.TrimSpace(ss[len(ss)-1])] = struct{}{}
	}
	for k := range f.InitialAdvertisePeerURLs {
		ss := strings.Split(k, ":")
		tm[strings.TrimSpace(ss[len(ss)-1])] = struct{}{}
	}
	sl := make([]string, len(tm))
	i := 0
	for k := range tm {
		sl[i] = k
		i++
	}
	sort.Strings(sl)
	return sl
}

func (f *Flags) getTag(fn string) (string, error) {
	field, ok := reflect.TypeOf(f).Elem().FieldByName(fn)
	if !ok {
		return "", fmt.Errorf("Field %s is not found!", fn)
	}
	return field.Tag.Get("flag"), nil
}

func getPairValueByName(fn, flags string) string {
	f := &Flags{}
	tag, _ := f.getTag(fn)
	// "--%s='%s'"
	prefix := fmt.Sprintf(`--%s='`, tag)
	rs := ""
	if strings.Contains(flags, prefix) {
		ss := strings.SplitN(flags, prefix, 2)
		if len(ss) > 1 {
			rs = strings.TrimSpace(strings.Split(ss[1], `' `)[0])
		}
	}
	return rs
}
