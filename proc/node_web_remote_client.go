package proc

import (
	"fmt"
	"sort"
	"sync"

	"github.com/coreos/etcd/tools/functional-tester/etcd-agent/client"
)

type NodeWebRemoteClient struct {
	mu    sync.Mutex
	Flags *Flags
	Agent client.Agent

	active bool
}

func (nd *NodeWebRemoteClient) Write(p []byte) (int, error) {
	return 0, nil // don't need (write on remote machines)
}

func (nd *NodeWebRemoteClient) GetGRPCAddr() string {
	return nd.Flags.ExperimentalgRPCAddr
}

func (nd *NodeWebRemoteClient) GetListenClientURLs() []string {
	ls := []string{}
	for k := range nd.Flags.ListenClientURLs {
		ls = append(ls, k)
	}
	sort.Strings(ls)
	return ls
}

func (nd *NodeWebRemoteClient) IsActive() bool {
	nd.mu.Lock()
	active := nd.active
	nd.mu.Unlock()
	return active
}

func (nd *NodeWebRemoteClient) Start() error {
	nd.mu.Lock()
	active := nd.active
	nd.mu.Unlock()
	if active {
		return fmt.Errorf("%s is already active", nd.Flags.Name)
	}

	flagSlice, err := nd.Flags.StringSlice()
	if err != nil {
		return err
	}
	if _, err := nd.Agent.Start(flagSlice...); err != nil {
		return err
	}
	nd.mu.Lock()
	nd.active = true
	nd.mu.Unlock()
	return nil
}

func (nd *NodeWebRemoteClient) Restart() error {
	nd.mu.Lock()
	active := nd.active
	nd.mu.Unlock()
	if active {
		return fmt.Errorf("%s is already active", nd.Flags.Name)
	}

	if _, err := nd.Agent.Restart(); err != nil {
		return err
	}
	nd.mu.Lock()
	nd.active = true
	nd.mu.Unlock()
	return nil
}

func (nd *NodeWebRemoteClient) Terminate() error {
	nd.mu.Lock()
	active := nd.active
	nd.mu.Unlock()
	if !active {
		return fmt.Errorf("%s is already terminated", nd.Flags.Name)
	}
	if err := nd.Agent.Stop(); err != nil {
		return err
	}
	nd.mu.Lock()
	nd.active = false
	nd.mu.Unlock()
	return nil
}

func (nd *NodeWebRemoteClient) Clean() error {
	if err := nd.Agent.Cleanup(); err != nil {
		return err
	}
	return nil
}
