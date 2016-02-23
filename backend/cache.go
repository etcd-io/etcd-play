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

package backend

import (
	"net/http"
	"sync"
	"time"

	"golang.org/x/net/context"

	"github.com/coreos/etcd-play/proc"
	"github.com/gorilla/websocket"
	"github.com/gyuho/psn/ss"
)

type (
	userData struct {
		upgrader *websocket.Upgrader

		startTime       time.Time
		lastRequestTime time.Time
		requestCount    int

		selectedNodeName  string
		selectedOperation string

		lastKey   string
		lastValue string

		keyHistory []string
	}

	cache struct {
		mu      sync.Mutex
		cluster proc.Cluster
		users   map[string]*userData
	}
)

var (
	globalPorts        = ss.NewPorts()
	globalCache *cache = nil
)

// initGlobalData must be called at the beginning of 'web' command.
func initGlobalData() {
	if globalFlags.LinuxAutoPort {
		globalPorts.Refresh()
		go func() {
			for {
				select {
				case <-time.After(globalFlags.LinuxIntervalPortRefresh):
					globalPorts.Refresh()
				}
			}
		}()
	}

	data := cache{
		cluster: nil,
		users:   make(map[string]*userData),
	}
	globalCache = &data

	globalCache.mu.Lock()
	if globalCache.users == nil {
		globalCache.users = make(map[string]*userData)
	}
	globalCache.mu.Unlock()

	go func() {
		for {
			now := time.Now()
			globalCache.mu.Lock()
			for userID, v := range globalCache.users {
				sub := now.Sub(v.startTime)
				// clean up users that started more than 1-hour ago
				if sub > time.Hour {
					delete(globalCache.users, userID)
				}
			}
			globalCache.mu.Unlock()

			time.Sleep(time.Hour)
		}
	}()
}

// checkCluster returns the cluster if the cluster is active.
func (s *cache) clusterActive() bool {
	s.mu.Lock()
	clu := s.cluster
	s.mu.Unlock()
	return clu != nil
}

func (s *cache) okToRequest(userID string) bool {
	s.mu.Lock()
	v, ok := s.users[userID]
	s.mu.Unlock()
	if !ok {
		return false
	}
	// allow maximum 5 requests per 2-second
	lastRequest := v.lastRequestTime
	if lastRequest.IsZero() {
		v.lastRequestTime = time.Now()
		v.requestCount = 1
		return true
	}
	v.requestCount++
	if v.requestCount < 5 {
		return true
	}
	sub := time.Now().Sub(lastRequest)
	if sub > 2*time.Second { // initialize
		v.lastRequestTime = time.Now()
		v.requestCount = 1
		return true
	}
	return false // count > 5 && sub < 2-sec
}

func withCache(h ContextHandler) ContextHandler {
	return ContextHandlerFunc(func(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
		userID := getUserID(req)
		ctx = context.WithValue(ctx, userKey, &userID)

		globalCache.mu.Lock()
		if _, ok := globalCache.users[userID]; !ok {
			globalCache.users[userID] = &userData{
				upgrader:  &websocket.Upgrader{},
				startTime: time.Now(),
				keyHistory: []string{
					`TYPE_YOUR_KEY`,
					`foo`,
					`sample_key`,
				},
			}
		}
		globalCache.mu.Unlock()

		// (X) this will deadlock
		// defer globalCache.mu.Unlock()
		return h.ServeHTTPContext(ctx, w, req)
	})
}
