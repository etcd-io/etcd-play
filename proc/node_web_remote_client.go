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
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/coreos/etcd/tools/functional-tester/etcd-agent/client"
)

type NodeWebRemoteClient struct {
	mu    sync.Mutex
	Flags *Flags
	Agent client.Agent

	active bool

	limitInterval  time.Duration
	lastTerminated time.Time
	lastRestarted  time.Time
}

func (nd *NodeWebRemoteClient) Endpoint() string {
	es := ""
	for k := range nd.Flags.ListenClientURLs {
		es = k
		break
	}
	s, _ := url.Parse(es)
	return s.Host
}

func (nd *NodeWebRemoteClient) StatusEndpoint() string {
	es := ""
	for k := range nd.Flags.ListenClientURLs {
		es = k
		break
	}
	return es
}

func (nd *NodeWebRemoteClient) IsActive() bool {
	nd.mu.Lock()
	defer nd.mu.Unlock()
	return nd.active
}

func (nd *NodeWebRemoteClient) Start() error {
	nd.mu.Lock()
	defer nd.mu.Unlock()

	if nd.active {
		return fmt.Errorf("%s is already running or requested to restart", nd.Flags.Name)
	}

	flagSlice, err := nd.Flags.StringSlice()
	if err != nil {
		return err
	}
	if _, err := nd.Agent.Start(flagSlice...); err != nil {
		return err
	}
	nd.active = true
	return nil
}

func (nd *NodeWebRemoteClient) Restart() error {
	nd.mu.Lock()
	defer nd.mu.Unlock()

	lastTerminated := nd.lastTerminated
	lastRestarted := nd.lastRestarted
	if nd.active {
		return fmt.Errorf("%s is already running or requested to restart", nd.Flags.Name)
	}

	// TODO: better way to wait resource release?
	// (we can scan the /proc to see if ports are still bind)
	//
	// restart, 2nd restart term should be more than limit interval
	sub := time.Now().Sub(lastRestarted)
	if sub < nd.limitInterval {
		return fmt.Errorf("Somebody restarted the same node (only %v ago)! Retry in %v!", sub, nd.limitInterval)
	}
	// terminate, and immediate restart term should be more than limit interval
	subt := time.Now().Sub(lastTerminated)
	if subt < nd.limitInterval {
		return fmt.Errorf("Somebody terminated the node (only %v ago)! Retry in %v!", subt, nd.limitInterval)
	}
	if _, err := nd.Agent.Restart(); err != nil {
		return err
	}

	nd.lastRestarted = time.Now()
	nd.active = true
	return nil
}

func (nd *NodeWebRemoteClient) Terminate() error {
	nd.mu.Lock()
	defer nd.mu.Unlock()

	lastTerminated := nd.lastTerminated
	lastRestarted := nd.lastRestarted
	if !nd.active {
		return fmt.Errorf("%s is already terminated or requested to terminate", nd.Flags.Name)
	}

	// terminate, 2nd terminate term should be more than limit interval
	sub := time.Now().Sub(lastTerminated)
	if sub < nd.limitInterval {
		return fmt.Errorf("Somebody terminated the same node (only %v ago)! Retry in %v!", sub, nd.limitInterval)
	}
	// restart, and immediate terminate term should be more than limit interval
	subt := time.Now().Sub(lastRestarted)
	if subt < nd.limitInterval {
		return fmt.Errorf("Somebody restarted the node (only %v ago)! Retry in %v!", subt, nd.limitInterval)
	}
	if err := nd.Agent.Stop(); err != nil {
		return err
	}

	nd.lastTerminated = time.Now()
	nd.active = false
	return nil
}

func (nd *NodeWebRemoteClient) Clean() error {
	if err := nd.Agent.Cleanup(); err != nil {
		return err
	}
	return nil
}
