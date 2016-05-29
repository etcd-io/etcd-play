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
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/coreos/etcd-play/proc"
	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
	"github.com/uber-go/zap"
	"golang.org/x/net/context"
)

type (
	key int

	Flags struct {
		EtcdBinary  string
		ClusterSize int
		LiveLog     bool

		KeepAlive      bool
		ClusterTimeout time.Duration
		LimitInterval  time.Duration
		ReviveInterval time.Duration

		StressNumber int

		PlayWebPort    string
		IsRemote       bool
		AgentEndpoints []string
	}
)

const (
	userKey key = 0
)

var (
	globalFlags = Flags{}
)

type ContextHandler interface {
	ServeHTTPContext(context.Context, http.ResponseWriter, *http.Request) error
}

type ContextHandlerFunc func(context.Context, http.ResponseWriter, *http.Request) error

func (f ContextHandlerFunc) ServeHTTPContext(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
	return f(ctx, w, req)
}

type ContextAdapter struct {
	ctx     context.Context
	handler ContextHandler
}

func (ca *ContextAdapter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if err := ca.handler.ServeHTTPContext(ca.ctx, w, req); err != nil {
		logger.Error("ServeHTTP",
			zap.String("method", req.Method),
			zap.String("path", req.URL.Path),
			zap.Err(err),
		)
	}
}

var (
	WebCommand = &cobra.Command{
		Use:   "web",
		Short: "web plays etcd in web browser.",
		Run:   CommandFunc,
	}
)

func init() {
	WebCommand.PersistentFlags().StringVarP(&globalFlags.EtcdBinary, "etcd-binary", "b", filepath.Join(os.Getenv("GOPATH"), "bin/etcd"), "path of executable etcd binary")
	WebCommand.PersistentFlags().IntVar(&globalFlags.ClusterSize, "cluster-size", 5, "size of cluster to create")
	WebCommand.PersistentFlags().BoolVar(&globalFlags.LiveLog, "live-log", false, "'true' to enable streaming etcd logs (only support localhost)")

	WebCommand.PersistentFlags().BoolVarP(&globalFlags.KeepAlive, "keep-alive", "k", false, "'true' to run demo without auto-termination (this overwrites cluster-timeout)")
	WebCommand.PersistentFlags().DurationVar(&globalFlags.ClusterTimeout, "cluster-timeout", 5*time.Minute, "after timeout, etcd shuts down the cluster")
	WebCommand.PersistentFlags().DurationVar(&globalFlags.LimitInterval, "limit-interval", 7*time.Second, "interval to rate-limit immediate restart, terminate")
	WebCommand.PersistentFlags().DurationVar(&globalFlags.ReviveInterval, "revive-interval", 15*time.Minute, "interval to automatically revive all-failed cluster")

	WebCommand.PersistentFlags().IntVar(&globalFlags.StressNumber, "stress-number", 3, "size of stress requests")

	WebCommand.PersistentFlags().StringVarP(&globalFlags.PlayWebPort, "port", "p", ":8000", "port to serve the play web interface")
	WebCommand.PersistentFlags().BoolVar(&globalFlags.IsRemote, "remote", false, "'true' when agents are deployed remotely")
	WebCommand.PersistentFlags().StringSliceVar(&globalFlags.AgentEndpoints, "agent-endpoints", []string{"localhost:9027"}, "list of remote agent endpoints")
}

func CommandFunc(cmd *cobra.Command, args []string) {
	defer func() {
		if err := recover(); err != nil {
			logger.Error("etcd-play error",
				zap.Object("error", err),
			)
			os.Exit(0)
		}
	}()

	if globalFlags.IsRemote {
		if globalFlags.ClusterSize != len(globalFlags.AgentEndpoints) {
			logger.Error("etcd-play cluster-size and agent-endpoints must be the same size",
				zap.Int("cluster-size", globalFlags.ClusterSize),
				zap.Object("agent-endpoints", len(globalFlags.AgentEndpoints)),
			)
			os.Exit(0)
		}
	}

	initGlobalData()

	rootContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mainRouter := http.NewServeMux()
	mainRouter.Handle("/", http.FileServer(http.Dir("./frontend")))

	mainRouter.Handle("/ws", &ContextAdapter{
		ctx:     rootContext,
		handler: withCache(ContextHandlerFunc(wsHandler)),
	})
	mainRouter.Handle("/stream", &ContextAdapter{
		ctx:     rootContext,
		handler: withCache(ContextHandlerFunc(streamHandler)),
	})
	mainRouter.Handle("/server_status", &ContextAdapter{
		ctx:     rootContext,
		handler: withCache(ContextHandlerFunc(serverStatusHandler)),
	})

	mainRouter.Handle("/start_cluster", &ContextAdapter{
		ctx:     rootContext,
		handler: withCache(ContextHandlerFunc(startClusterHandler)),
	})

	mainRouter.Handle("/stress", &ContextAdapter{
		ctx:     rootContext,
		handler: withCache(ContextHandlerFunc(stressHandler)),
	})

	mainRouter.Handle("/key_history", &ContextAdapter{
		ctx:     rootContext,
		handler: withCache(ContextHandlerFunc(keyHistoryHandler)),
	})
	mainRouter.Handle("/key_value", &ContextAdapter{
		ctx:     rootContext,
		handler: withCache(ContextHandlerFunc(keyValueHandler)),
	})

	mainRouter.Handle("/kill_1", &ContextAdapter{
		ctx:     rootContext,
		handler: withCache(ContextHandlerFunc(killHandler)),
	})
	mainRouter.Handle("/kill_2", &ContextAdapter{
		ctx:     rootContext,
		handler: withCache(ContextHandlerFunc(killHandler)),
	})
	mainRouter.Handle("/kill_3", &ContextAdapter{
		ctx:     rootContext,
		handler: withCache(ContextHandlerFunc(killHandler)),
	})
	mainRouter.Handle("/kill_4", &ContextAdapter{
		ctx:     rootContext,
		handler: withCache(ContextHandlerFunc(killHandler)),
	})
	mainRouter.Handle("/kill_5", &ContextAdapter{
		ctx:     rootContext,
		handler: withCache(ContextHandlerFunc(killHandler)),
	})

	mainRouter.Handle("/restart_1", &ContextAdapter{
		ctx:     rootContext,
		handler: withCache(ContextHandlerFunc(restartHandler)),
	})
	mainRouter.Handle("/restart_2", &ContextAdapter{
		ctx:     rootContext,
		handler: withCache(ContextHandlerFunc(restartHandler)),
	})
	mainRouter.Handle("/restart_3", &ContextAdapter{
		ctx:     rootContext,
		handler: withCache(ContextHandlerFunc(restartHandler)),
	})
	mainRouter.Handle("/restart_4", &ContextAdapter{
		ctx:     rootContext,
		handler: withCache(ContextHandlerFunc(restartHandler)),
	})
	mainRouter.Handle("/restart_5", &ContextAdapter{
		ctx:     rootContext,
		handler: withCache(ContextHandlerFunc(restartHandler)),
	})

	logger.Info("started serving", zap.String("address", fmt.Sprintf("http://localhost%s", globalFlags.PlayWebPort)))
	if err := http.ListenAndServe(globalFlags.PlayWebPort, mainRouter); err != nil {
		logger.Error("etcd-play error",
			zap.Object("error", err),
		)
		os.Exit(0)
	}
}

// wsHandler monitors user activities and notifies when a user leaves the web pages.
func wsHandler(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
	user := ctx.Value(userKey).(*string)
	userID := *user

	globalCache.mu.Lock()
	upgrader := globalCache.users[userID].upgrader
	globalCache.mu.Unlock()

	c, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		// clean up users that just left the browser
		globalCache.mu.Lock()
		delete(globalCache.users, userID)
		globalCache.mu.Unlock()
		return err
	}
	defer func() {
		globalCache.mu.Lock()
		if c != nil {
			c.Close()
		}
		globalCache.mu.Unlock()
	}()

	for {
		mt, message, err := c.ReadMessage()
		if err != nil {
			globalCache.mu.Lock()
			delete(globalCache.users, userID)
			globalCache.mu.Unlock()
			return err
		}
		if err := c.WriteMessage(mt, message); err != nil {
			globalCache.mu.Lock()
			delete(globalCache.users, userID)
			globalCache.mu.Unlock()
			return err
		}
	}
}

func streamHandler(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
	user := ctx.Value(userKey).(*string)
	userID := *user

	switch req.Method {
	case "GET":
		if !globalCache.clusterActive() {
			fmt.Fprintln(w, boldHTMLMsg("Cluster is not active... Please start the cluster..."))
			return nil
		}
		globalCache.mu.Lock()
		sharedStream := globalCache.cluster.SharedStream()
		userStream := globalCache.cluster.Stream(userID)
		globalCache.mu.Unlock()

		// no need Lock because it's channel
		//
		// globalCache.mu.Lock()
		// cluster.Stream()
		// globalCache.mu.Unlock()

		streams := []string{}

	escape:
		for {
			select {
			case s := <-sharedStream:
				streams = append(streams, s)

			case s := <-userStream:
				streams = append(streams, s)

			case <-time.After(time.Second):
				// drain channel until it takes longer than 1 second
				break escape
			}
		}

		if len(streams) == 0 {
			return nil
		}

		// When used with new EventSource('/stream') in Javascript
		//
		// w.Header().Set("Content-Type", "text/event-stream")
		// fmt.Fprintf(w, fmt.Sprintf("id: %s\nevent: %s\ndata: %s\n\n", userID, "stream_log", strings.Join(streams, "\n")))

		// When used with setInterval in Javascript
		//
		// fmt.Fprintln(w, strings.Join(streams, "<br>"))

		resp := struct {
			Logs string
			Size int
		}{
			strings.Join(streams, "<br>"),
			len(streams),
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			return err
		}

		if f, ok := w.(http.Flusher); ok {
			if f != nil {
				f.Flush()
			}
		}

	default:
		http.Error(w, "Method Not Allowed", 405)
	}

	return nil
}

func startClusterHandler(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
	user := ctx.Value(userKey).(*string)
	userID := *user

	nodeType := proc.WebLocal
	if globalFlags.IsRemote {
		nodeType = proc.WebRemote
	}

	switch req.Method {
	case "GET":
		wport := globalFlags.PlayWebPort
		if globalFlags.IsRemote {
			wport = ":80"
		}
		resp := struct {
			Message     string
			PlayWebPort string
		}{
			getWelcomeMsg(),
			wport,
		}

		if globalCache.clusterActive() {
			resp.Message += boldHTMLMsg("Cluster is already started! Loading the cluster information...")
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				return err
			}
			return nil
		}

		done, errc := make(chan struct{}), make(chan error)
		go startCluster(nodeType, globalFlags.ClusterSize, globalFlags.LiveLog, globalFlags.LimitInterval, globalFlags.AgentEndpoints, userID, done, errc)
		select {
		case <-done:
			resp.Message += boldHTMLMsg("Start cluster successfully requested!!!")
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				return err
			}
		case err := <-errc:
			resp.Message += boldHTMLMsg(fmt.Sprintf("Start cluster failed (%v)", err))
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				return err
			}
		}

	default:
		http.Error(w, "Method Not Allowed", 405)
	}

	return nil
}

func startCluster(nodeType proc.NodeType, clusterSize int, liveLog bool, limitInterval time.Duration, agentEndpoints []string, userID string, done chan struct{}, errc chan error) {
	fs := make([]*proc.Flags, clusterSize)
	for i := range fs {
		host := "localhost"
		if len(agentEndpoints) > 1 && nodeType == proc.WebRemote {
			host = strings.TrimSpace(strings.Split(agentEndpoints[i], ":")[0])
		}
		df, err := proc.GenerateFlags(fmt.Sprintf("etcd%d", i+1), host, nodeType == proc.WebRemote)
		if err != nil {
			errc <- err
			return
		}
		fs[i] = df
	}

	opts := []proc.OpOption{proc.WithLimitInterval(limitInterval), proc.WithAgentEndpoints(agentEndpoints)}
	if liveLog {
		opts = append(opts, proc.WithLiveLog())
	}
	c, err := proc.NewCluster(nodeType, globalFlags.EtcdBinary, fs, opts...)
	if err != nil {
		errc <- err
		return
	}

	if globalCache.clusterActive() {
		done <- struct{}{}
		return
	}
	globalCache.mu.Lock()
	globalCache.cluster = c
	globalCache.mu.Unlock()

	// this does not run with the program exits with os.Exit(0)
	defer func() {
		if !globalFlags.KeepAlive {
			c.Shutdown()
			globalCache.mu.Lock()
			globalCache.cluster = nil
			globalCache.mu.Unlock()
		}
	}()

	cdone, cerr := make(chan struct{}), make(chan error)
	go func() {
		defer func() {
			cdone <- struct{}{}
		}()
		c.Stream(userID) <- boldHTMLMsg(fmt.Sprintf("Starting %d nodes", globalFlags.ClusterSize))
		if err := c.Bootstrap(); err != nil {
			cerr <- err
			return
		}
	}()
	done <- struct{}{}

	select {
	case err := <-cerr:
		c.Stream(userID) <- boldHTMLMsg(fmt.Sprintf("Cluster error(%v)", err))

	case <-cdone:
		c.Stream(userID) <- boldHTMLMsg("Cluster exited from an unexpected interruption!")

	case <-time.After(globalFlags.ClusterTimeout):
		if !globalFlags.KeepAlive {
			c.Stream(userID) <- boldHTMLMsg(fmt.Sprintf("Cluster time out (%v)! Please restart the cluster.", globalFlags.ClusterTimeout))
		}
	}
}

func serverStatusHandler(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
	switch req.Method {
	case "GET":
		if !globalCache.clusterActive() {
			return nil
		}

		globalStatus.mu.RLock()
		activeUserList := globalStatus.activeUserList
		copiedNameToStatus := make(map[string]proc.ServerStatus)
		for k, v := range globalStatus.nameToStatus {
			copiedNameToStatus[k] = v
		}
		globalStatus.mu.RUnlock()

		etcd1_ID, etcd1_Endpoint, etcd1_State := "unknown", "unknown", ""
		etcd1_DbSize, etcd1_DbSizeTxt, etcd1_Hash := uint64(0), "0 B", 0
		if v, ok := copiedNameToStatus["etcd1"]; ok {
			etcd1_ID = v.ID
			etcd1_Endpoint = v.Endpoint
			etcd1_State = v.State
			etcd1_Hash = v.Hash
			etcd1_DbSize = v.DbSize
			etcd1_DbSizeTxt = v.DbSizeTxt
		}
		etcd2_ID, etcd2_Endpoint, etcd2_State := "unknown", "unknown", ""
		etcd2_DbSize, etcd2_DbSizeTxt, etcd2_Hash := uint64(0), "0 B", 0
		if v, ok := copiedNameToStatus["etcd2"]; ok {
			etcd2_ID = v.ID
			etcd2_Endpoint = v.Endpoint
			etcd2_State = v.State
			etcd2_Hash = v.Hash
			etcd2_DbSize = v.DbSize
			etcd2_DbSizeTxt = v.DbSizeTxt
		}
		etcd3_ID, etcd3_Endpoint, etcd3_State := "unknown", "unknown", ""
		etcd3_DbSize, etcd3_DbSizeTxt, etcd3_Hash := uint64(0), "0 B", 0
		if v, ok := copiedNameToStatus["etcd3"]; ok {
			etcd3_ID = v.ID
			etcd3_Endpoint = v.Endpoint
			etcd3_State = v.State
			etcd3_Hash = v.Hash
			etcd3_DbSize = v.DbSize
			etcd3_DbSizeTxt = v.DbSizeTxt
		}
		etcd4_ID, etcd4_Endpoint, etcd4_State := "unknown", "unknown", ""
		etcd4_DbSize, etcd4_DbSizeTxt, etcd4_Hash := uint64(0), "0 B", 0
		if v, ok := copiedNameToStatus["etcd4"]; ok {
			etcd4_ID = v.ID
			etcd4_Endpoint = v.Endpoint
			etcd4_State = v.State
			etcd4_Hash = v.Hash
			etcd4_DbSize = v.DbSize
			etcd4_DbSizeTxt = v.DbSizeTxt
		}
		etcd5_ID, etcd5_Endpoint, etcd5_State := "unknown", "unknown", ""
		etcd5_DbSize, etcd5_DbSizeTxt, etcd5_Hash := uint64(0), "0 B", 0
		if v, ok := copiedNameToStatus["etcd5"]; ok {
			etcd5_ID = v.ID
			etcd5_Endpoint = v.Endpoint
			etcd5_State = v.State
			etcd5_Hash = v.Hash
			etcd5_DbSize = v.DbSize
			etcd5_DbSizeTxt = v.DbSizeTxt
		}

		resp := struct {
			ServerUptime     string
			ActiveUserNumber int
			ActiveUserList   string

			Etcd1_Name      string
			Etcd1_ID        string
			Etcd1_Endpoint  string
			Etcd1_State     string
			Etcd1_Hash      int
			Etcd1_DbSize    uint64
			Etcd1_DbSizeTxt string

			Etcd2_Name      string
			Etcd2_ID        string
			Etcd2_Endpoint  string
			Etcd2_State     string
			Etcd2_Hash      int
			Etcd2_DbSize    uint64
			Etcd2_DbSizeTxt string

			Etcd3_Name      string
			Etcd3_ID        string
			Etcd3_Endpoint  string
			Etcd3_State     string
			Etcd3_Hash      int
			Etcd3_DbSize    uint64
			Etcd3_DbSizeTxt string

			Etcd4_Name      string
			Etcd4_ID        string
			Etcd4_Endpoint  string
			Etcd4_State     string
			Etcd4_Hash      int
			Etcd4_DbSize    uint64
			Etcd4_DbSizeTxt string

			Etcd5_Name      string
			Etcd5_ID        string
			Etcd5_Endpoint  string
			Etcd5_State     string
			Etcd5_Hash      int
			Etcd5_DbSize    uint64
			Etcd5_DbSizeTxt string
		}{
			humanize.Time(startTime),
			len(globalCache.users),
			activeUserList,

			"etcd1",
			etcd1_ID,
			etcd1_Endpoint,
			etcd1_State,
			etcd1_Hash,
			etcd1_DbSize,
			etcd1_DbSizeTxt,

			"etcd2",
			etcd2_ID,
			etcd2_Endpoint,
			etcd2_State,
			etcd2_Hash,
			etcd2_DbSize,
			etcd2_DbSizeTxt,

			"etcd3",
			etcd3_ID,
			etcd3_Endpoint,
			etcd3_State,
			etcd3_Hash,
			etcd3_DbSize,
			etcd3_DbSizeTxt,

			"etcd4",
			etcd4_ID,
			etcd4_Endpoint,
			etcd4_State,
			etcd4_Hash,
			etcd4_DbSize,
			etcd4_DbSizeTxt,

			"etcd5",
			etcd5_ID,
			etcd5_Endpoint,
			etcd5_State,
			etcd5_Hash,
			etcd5_DbSize,
			etcd5_DbSizeTxt,
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			return err
		}

	default:
		http.Error(w, "Method Not Allowed", 405)
	}

	return nil
}

func stressHandler(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
	user := ctx.Value(userKey).(*string)
	userID := *user

	switch req.Method {
	case "POST":
		if !globalCache.okToRequest(userID) {
			fmt.Fprintln(w, boldHTMLMsg("Rate limit excess! Please retry..."))
			return nil
		}
		if err := req.ParseForm(); err != nil {
			fmt.Fprintln(w, boldHTMLMsg(fmt.Sprintf("error: %v", err)))
			return err
		}
		selectedNodeName := ""
		if len(req.Form["selected_node_name"]) != 0 {
			selectedNodeName = req.Form["selected_node_name"][0]
		}
		globalCache.mu.Lock()
		globalCache.users[userID].selectedNodeName = selectedNodeName
		globalCache.mu.Unlock()

	case "GET":
		if !globalCache.clusterActive() {
			fmt.Fprintln(w, boldHTMLMsg("Cluster is not active... Please start the cluster..."))
			return nil
		}
		if !globalCache.okToRequest(userID) {
			fmt.Fprintln(w, boldHTMLMsg("Rate limit excess! Please retry..."))
			return nil
		}

		globalCache.mu.Lock()
		selectedNodeName := globalCache.users[userID].selectedNodeName
		cluster := globalCache.cluster
		globalCache.mu.Unlock()

		took, err := cluster.Stress(selectedNodeName, globalFlags.StressNumber, userID)
		if err != nil {
			fmt.Fprintln(w, boldHTMLMsg(fmt.Sprintf("error: %v", err)))
			return err
		}

		rs := fmt.Sprintf("Success! Wrote %d keys to %q (took %v)", globalFlags.StressNumber, selectedNodeName, took)
		resp := struct {
			Message string
			Result  string
		}{
			boldHTMLMsg("[STRESS] Success!"),
			"<b>[STRESS]</b> " + rs,
		}
		if err = json.NewEncoder(w).Encode(resp); err != nil {
			return err
		}

	default:
		http.Error(w, "Method Not Allowed", 405)
	}

	return nil
}

func keyHistoryHandler(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
	user := ctx.Value(userKey).(*string)
	userID := *user

	switch req.Method {
	case "GET":
		globalCache.mu.Lock()
		keyHistory := globalCache.users[userID].keyHistory
		globalCache.mu.Unlock()

		resp := struct {
			Values []string
		}{
			keyHistory,
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			return err
		}

	default:
		http.Error(w, "Method Not Allowed", 405)
	}

	return nil
}

func keyValueHandler(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
	user := ctx.Value(userKey).(*string)
	userID := *user

	switch req.Method {
	case "POST":
		if !globalCache.okToRequest(userID) {
			fmt.Fprintln(w, boldHTMLMsg("Rate limit excess! Please retry..."))
			return nil
		}

		if err := req.ParseForm(); err != nil {
			return err
		}
		selectedNodeName := ""
		if len(req.Form["selected_node_name"]) != 0 {
			selectedNodeName = req.Form["selected_node_name"][0]
		}
		selectedOperation := ""
		if len(req.Form["selected_operation"]) != 0 {
			selectedOperation = req.Form["selected_operation"][0]
		}
		key := ""
		if len(req.Form["key_input"]) != 0 {
			key = template.HTMLEscapeString(req.Form["key_input"][0])
		}
		if len(key) > 100 { // truncate user-input
			key = key[:100]
		}
		value := ""
		if len(req.Form["value_input"]) != 0 {
			value = template.HTMLEscapeString(req.Form["value_input"][0])
		}
		if len(value) > 200 { // truncate user-input
			value = value[:200]
		}

		globalCache.mu.Lock()
		globalCache.users[userID].selectedOperation = selectedOperation
		globalCache.users[userID].selectedNodeName = selectedNodeName
		globalCache.users[userID].lastKey = key
		globalCache.users[userID].lastValue = value
		if key != "" {
			hm := make(map[string]struct{})
			for _, v := range globalCache.users[userID].keyHistory {
				hm[v] = struct{}{}
			}
			if _, ok := hm[key]; !ok {
				globalCache.users[userID].keyHistory = append(globalCache.users[userID].keyHistory, key)
			}
		}
		history := globalCache.users[userID].keyHistory
		if len(history) > 7 { // FIFO of at most 7 command histories
			copied := make([]string, 7)
			copy(copied, history[1:])
			history = copied
		}
		globalCache.users[userID].keyHistory = history
		globalCache.mu.Unlock()

	case "GET":
		if !globalCache.okToRequest(userID) {
			fmt.Fprintln(w, boldHTMLMsg("Rate limit excess! Please retry..."))
			return nil
		}
		globalCache.mu.Lock()
		cluster := globalCache.cluster
		opt := globalCache.users[userID].selectedOperation
		name := globalCache.users[userID].selectedNodeName
		key := globalCache.users[userID].lastKey
		value := globalCache.users[userID].lastValue
		globalCache.mu.Unlock()

		switch opt {
		case "PUT":
			took, err := cluster.Put(name, key, value, userID)
			if err != nil {
				resp := struct {
					Message string
					Result  string
				}{
					boldHTMLMsg(fmt.Sprintf("[PUT] error %v (key %q / value %q)", err, key, value)),
					fmt.Sprintf("<b>[PUT] error %v (key %q)</b>", err, key),
				}
				if err = json.NewEncoder(w).Encode(resp); err != nil {
					return err
				}
			} else {
				keyT, valT := key, value
				if len(keyT) > 3 {
					keyT = keyT[:3] + "..."
				}
				if len(valT) > 3 {
					valT = valT[:3] + "..."
				}
				rs := fmt.Sprintf("Success! %q : %q (took %v)", keyT, valT, took)
				resp := struct {
					Message string
					Result  string
				}{
					boldHTMLMsg("[PUT] Success!"),
					"<b>[PUT]</b> " + rs,
				}
				if err = json.NewEncoder(w).Encode(resp); err != nil {
					return err
				}
			}

		case "GET":
			keyTxt, prefix := strings.TrimSpace(key), false
			if strings.Contains(keyTxt, "--prefix") {
				keyTxt = strings.TrimSpace(strings.Replace(keyTxt, "--prefix", "", 1))
				prefix = true
			}
			vs, took, err := cluster.Get(name, keyTxt, prefix, userID)
			if err != nil {
				resp := struct {
					Message string
					Result  string
				}{
					boldHTMLMsg(fmt.Sprintf("[GET] error %v (key %q)", err, keyTxt)),
					fmt.Sprintf("<b>[GET] error %v (key %q)</b>", err, keyTxt),
				}
				if err = json.NewEncoder(w).Encode(resp); err != nil {
					return err
				}
			} else {
				ks := keyTxt
				if len(ks) == 0 {
					ks = "\x00"
				}
				res := ""
				for i, rv := range vs {
					res += fmt.Sprintf("%q", rv)
					if i != len(vs)-1 {
						res += ", "
					}
					if i > 0 && i%5 == 0 {
						res += "<br>"
					}
					if i > 25 {
						res += "... (see below)"
						break
					}
				}
				rs := fmt.Sprintf("<b>[GET]</b> %s (key %q, took %v)", res, ks, took)
				if len(vs) == 0 {
					rs = fmt.Sprintf("<b>[GET]</b> not exist (key %q, took %v)", ks, took)
				}
				resp := struct {
					Message string
					Result  string
				}{
					boldHTMLMsg("[GET] Success!"),
					rs,
				}
				if err = json.NewEncoder(w).Encode(resp); err != nil {
					return err
				}
			}

		case "DELETE":
			keyTxt, prefix := strings.TrimSpace(key), false
			if strings.Contains(keyTxt, "--prefix") {
				keyTxt = strings.TrimSpace(strings.Replace(keyTxt, "--prefix", "", 1))
				prefix = true
			}
			delN, took, err := cluster.Delete(name, keyTxt, prefix, userID)
			if err != nil {
				ks := keyTxt
				if len(ks) == 0 {
					ks = "\x00"
				}
				resp := struct {
					Message string
					Result  string
				}{
					boldHTMLMsg(fmt.Sprintf("[DELETE] error %v (key %q)", err, ks)),
					fmt.Sprintf("<b>[DELETE] error %v (key %q)</b>", err, ks),
				}
				if err = json.NewEncoder(w).Encode(resp); err != nil {
					return err
				}
			} else {
				ks := keyTxt
				if len(ks) == 0 {
					ks = "\x00"
				}
				resp := struct {
					Message string
					Result  string
				}{
					boldHTMLMsg("[DELETE] Success!"),
					fmt.Sprintf("<b>[DELETE]</b> successfully deleted %q (deleted %d keys, took %v)", ks, delN, took),
				}
				if err = json.NewEncoder(w).Encode(resp); err != nil {
					return err
				}
			}
		}

	default:
		http.Error(w, "Method Not Allowed", 405)
	}

	return nil
}

func killHandler(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
	user := ctx.Value(userKey).(*string)
	userID := *user

	switch req.Method {
	case "GET":
		if !globalCache.clusterActive() {
			fmt.Fprintln(w, boldHTMLMsg("Cluster is not active... Please start the cluster..."))
			return nil
		}
		if !globalCache.okToRequest(userID) {
			fmt.Fprintln(w, boldHTMLMsg("Rate limit excess! Please retry..."))
			return nil
		}

		globalCache.mu.Lock()
		defer globalCache.mu.Unlock()

		name := urlToName(req.URL.String())
		if err := globalCache.cluster.Terminate(name); err != nil {
			fmt.Fprintln(w, boldHTMLMsg(fmt.Sprintf("error: %v", err)))
			return err
		}
		fmt.Fprintln(w, boldHTMLMsg(fmt.Sprintf("Kill %s request successfully requested", name)))

	default:
		http.Error(w, "Method Not Allowed", 405)
	}

	return nil
}

func restartHandler(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
	user := ctx.Value(userKey).(*string)
	userID := *user

	switch req.Method {
	case "GET":
		if !globalCache.clusterActive() {
			fmt.Fprintln(w, boldHTMLMsg("Cluster is not active... Please start the cluster..."))
			return nil
		}
		if !globalCache.okToRequest(userID) {
			fmt.Fprintln(w, boldHTMLMsg("Rate limit excess! Please retry..."))
			return nil
		}

		globalCache.mu.Lock()
		defer globalCache.mu.Unlock()

		name := urlToName(req.URL.String())
		if err := globalCache.cluster.Restart(name); err != nil {
			fmt.Fprintln(w, boldHTMLMsg(fmt.Sprintf("error: %v", err)))
			return err
		}
		fmt.Fprintln(w, boldHTMLMsg(fmt.Sprintf("Restart %s request successfully requested", name)))

	default:
		http.Error(w, "Method Not Allowed", 405)
	}

	return nil
}
