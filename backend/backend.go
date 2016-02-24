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
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/coreos/etcd-play/proc"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

type (
	key int

	Flags struct {
		EtcdBinary     string
		ClusterSize    int
		DisableLiveLog bool

		LinuxAutoPort            bool
		LinuxIntervalPortRefresh time.Duration

		KeepAlive      bool
		ClusterTimeout time.Duration

		StressNumber int

		PlayWebPort string
		Production  bool

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

		// import log "github.com/Sirupsen/logrus"
		// log.WithFields(log.Fields{
		// 	"event_type": "error",
		// 	"method":     req.Method,
		// 	"path":       req.URL.Path,
		// 	"error":      err,
		// }).Errorln("ServeHTTP error")

		log.Printf("[ServeHTTP error] %v (method: %s, path: %s)", err, req.Method, req.URL.Path)
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
	WebCommand.PersistentFlags().BoolVar(&globalFlags.DisableLiveLog, "disable-live-log", false, "'true' to disable streaming etcd logs")

	WebCommand.PersistentFlags().BoolVar(&globalFlags.LinuxAutoPort, "linux-auto-port", strings.Contains(runtime.GOOS, "linux"), "(only linux supported) 'true' to automate port findings")
	WebCommand.PersistentFlags().DurationVar(&globalFlags.LinuxIntervalPortRefresh, "linux-port-refresh", 10*time.Second, "(only linux supported) interval to refresh free ports")

	WebCommand.PersistentFlags().BoolVar(&globalFlags.KeepAlive, "keep-alive", false, "'true' to run demo without auto-termination (this overwrites cluster-timeout)")
	WebCommand.PersistentFlags().DurationVar(&globalFlags.ClusterTimeout, "cluster-timeout", 5*time.Minute, "after timeout, etcd shuts down the cluster")

	WebCommand.PersistentFlags().IntVar(&globalFlags.StressNumber, "stress-number", 10, "size of stress requests")

	WebCommand.PersistentFlags().StringVarP(&globalFlags.PlayWebPort, "port", "p", ":8000", "port to serve the play web interface")
	WebCommand.PersistentFlags().BoolVar(&globalFlags.Production, "production", false, "'true' when deploying as a web server in production")

	WebCommand.PersistentFlags().BoolVar(&globalFlags.IsRemote, "remote", false, "'true' when agents are deployed remotely")
	WebCommand.PersistentFlags().StringSliceVar(&globalFlags.AgentEndpoints, "agent-endpoints", []string{"localhost:9027"}, "list of remote agent endpoints")
}

func CommandFunc(cmd *cobra.Command, args []string) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Fprintln(os.Stdout, "[web]", err)
			os.Exit(0)
		}
	}()

	if globalFlags.IsRemote {
		if globalFlags.ClusterSize != len(globalFlags.AgentEndpoints) {
			fmt.Fprintf(os.Stdout, "[etcd-play error] cluster-size and agent-endpoints must be the same size (%d != %d)\n", globalFlags.ClusterSize, len(globalFlags.AgentEndpoints))
			os.Exit(0)
		}
	}

	initGlobalData()

	rootContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mainRouter := http.NewServeMux()

	// mainRouter.Handle("/", http.FileServer(http.Dir("./frontend")))
	staticHandler := staticLocalHandler
	if globalFlags.IsRemote {
		staticHandler = staticRemoteHandler
	}
	mainRouter.Handle("/", &ContextAdapter{
		ctx:     rootContext,
		handler: withCache(ContextHandlerFunc(staticHandler)),
	})
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

	fmt.Fprintln(os.Stdout, "Serving http://localhost"+globalFlags.PlayWebPort)
	if err := http.ListenAndServe(globalFlags.PlayWebPort, mainRouter); err != nil {
		fmt.Fprintln(os.Stdout, "[etcd-play error]", err)
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
		if globalFlags.Production {
			wport = ":80"
		}
		resp := struct {
			Message     string
			PlayWebPort string
		}{
			boldHTMLMsg("Hello World! Welcome to etcd!") + `<br>
- Please click the <font color='blue'>circle(node)</font> for more node information.<br>
- <font color='red'>Kill</font> to stop the node. <font color='red'>Restart</font> to recover the node.<br>
- You can even <font color='red'>kill</font> the <font color='green'>leader</font>!<br>
- <font color='blue'>Hash</font> shows how <b>etcd</b>, <i>as a distributed database</i>, <b>keeps its consistency</b>.<br>
- Select <b>any endpoint</b><i>(etcd1, etcd2, ...)</i> to PUT, GET, DELETE, and then click <b>Submit</b>.<br>
<br>
<i>Note: Since we do not ask for users' identities, all request logs are streamed<br>
based on your IP and user agents. So if you have multiple browsers running this<br>
same web page, logs could be shown only in one of them.</i><br>
<br>
If there's any issue, please contact us at https://github.com/coreos/etcd-play/issues.<br>
Thanks and enjoy!<br>
`,
			wport,
		}

		if globalCache.clusterActive() {
			resp.Message += boldHTMLMsg("Cluster is already started!")
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				return err
			}
			return nil
		}

		done, errc := make(chan struct{}), make(chan error)
		go startCluster(nodeType, globalFlags.ClusterSize, globalFlags.DisableLiveLog, globalFlags.AgentEndpoints, userID, done, errc)
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

func startCluster(nodeType proc.NodeType, clusterSize int, disableLiveLog bool, agentEndpoints []string, userID string, done chan struct{}, errc chan error) {
	fs := make([]*proc.Flags, clusterSize)
	for i := range fs {
		host := "localhost"
		if len(agentEndpoints) > 1 && nodeType == proc.WebRemote {
			host = strings.TrimSpace(strings.Split(agentEndpoints[i], ":")[0])
		}
		df, err := proc.GenerateFlags(fmt.Sprintf("etcd%d", i+1), host, nodeType == proc.WebRemote, globalPorts)
		if err != nil {
			errc <- err
			return
		}
		fs[i] = df
	}

	c, err := proc.NewCluster(nodeType, disableLiveLog, agentEndpoints, globalFlags.EtcdBinary, fs...)
	if err != nil {
		errc <- err
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

		globalStatus.mu.Lock()
		etcd1_ID, etcd1_Endpoint, etcd1_State := "unknown", "unknown", ""
		etcd1_NumberOfKeys, etcd1_Hash := 0, 0
		if v, ok := globalStatus.nameToStatus["etcd1"]; ok {
			etcd1_ID = v.ID
			etcd1_Endpoint = v.Endpoint
			etcd1_State = v.State
			etcd1_NumberOfKeys = v.NumberOfKeys
			etcd1_Hash = v.Hash
		}
		etcd2_ID, etcd2_Endpoint, etcd2_State := "unknown", "unknown", ""
		etcd2_NumberOfKeys, etcd2_Hash := 0, 0
		if v, ok := globalStatus.nameToStatus["etcd2"]; ok {
			etcd2_ID = v.ID
			etcd2_Endpoint = v.Endpoint
			etcd2_State = v.State
			etcd2_NumberOfKeys = v.NumberOfKeys
			etcd2_Hash = v.Hash
		}
		etcd3_ID, etcd3_Endpoint, etcd3_State := "unknown", "unknown", ""
		etcd3_NumberOfKeys, etcd3_Hash := 0, 0
		if v, ok := globalStatus.nameToStatus["etcd3"]; ok {
			etcd3_ID = v.ID
			etcd3_Endpoint = v.Endpoint
			etcd3_State = v.State
			etcd3_NumberOfKeys = v.NumberOfKeys
			etcd3_Hash = v.Hash
		}
		etcd4_ID, etcd4_Endpoint, etcd4_State := "unknown", "unknown", ""
		etcd4_NumberOfKeys, etcd4_Hash := 0, 0
		if v, ok := globalStatus.nameToStatus["etcd4"]; ok {
			etcd4_ID = v.ID
			etcd4_Endpoint = v.Endpoint
			etcd4_State = v.State
			etcd4_NumberOfKeys = v.NumberOfKeys
			etcd4_Hash = v.Hash
		}
		etcd5_ID, etcd5_Endpoint, etcd5_State := "unknown", "unknown", ""
		etcd5_NumberOfKeys, etcd5_Hash := 0, 0
		if v, ok := globalStatus.nameToStatus["etcd5"]; ok {
			etcd5_ID = v.ID
			etcd5_Endpoint = v.Endpoint
			etcd5_State = v.State
			etcd5_NumberOfKeys = v.NumberOfKeys
			etcd5_Hash = v.Hash
		}
		globalStatus.mu.Unlock()

		resp := struct {
			ActiveUsers int

			Etcd1_Name         string
			Etcd1_ID           string
			Etcd1_Endpoint     string
			Etcd1_State        string
			Etcd1_NumberOfKeys int
			Etcd1_Hash         int

			Etcd2_Name         string
			Etcd2_ID           string
			Etcd2_Endpoint     string
			Etcd2_State        string
			Etcd2_NumberOfKeys int
			Etcd2_Hash         int

			Etcd3_Name         string
			Etcd3_ID           string
			Etcd3_Endpoint     string
			Etcd3_State        string
			Etcd3_NumberOfKeys int
			Etcd3_Hash         int

			Etcd4_Name         string
			Etcd4_ID           string
			Etcd4_Endpoint     string
			Etcd4_State        string
			Etcd4_NumberOfKeys int
			Etcd4_Hash         int

			Etcd5_Name         string
			Etcd5_ID           string
			Etcd5_Endpoint     string
			Etcd5_State        string
			Etcd5_NumberOfKeys int
			Etcd5_Hash         int
		}{
			len(globalCache.users),

			"etcd1",
			etcd1_ID,
			etcd1_Endpoint,
			etcd1_State,
			etcd1_NumberOfKeys,
			etcd1_Hash,

			"etcd2",
			etcd2_ID,
			etcd2_Endpoint,
			etcd2_State,
			etcd2_NumberOfKeys,
			etcd2_Hash,

			"etcd3",
			etcd3_ID,
			etcd3_Endpoint,
			etcd3_State,
			etcd3_NumberOfKeys,
			etcd3_Hash,

			"etcd4",
			etcd4_ID,
			etcd4_Endpoint,
			etcd4_State,
			etcd4_NumberOfKeys,
			etcd4_Hash,

			"etcd5",
			etcd5_ID,
			etcd5_Endpoint,
			etcd5_State,
			etcd5_NumberOfKeys,
			etcd5_Hash,
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
			fmt.Fprintln(w, boldHTMLMsg(fmt.Sprintf("error (%v)", err)))
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

		if err := cluster.Stress(selectedNodeName, 10); err != nil {
			fmt.Fprintln(w, boldHTMLMsg(fmt.Sprintf("error (%v)", err)))
			return err
		}
		fmt.Fprintln(w, boldHTMLMsg(fmt.Sprintf("Stress %s request successfully requested", selectedNodeName)))

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
			key = req.Form["key_input"][0]
		}
		value := ""
		if len(req.Form["value_input"]) != 0 {
			value = req.Form["value_input"][0]
		}

		globalCache.mu.Lock()
		globalCache.users[userID].selectedOperation = selectedOperation
		globalCache.users[userID].selectedNodeName = selectedNodeName
		globalCache.users[userID].lastKey = key
		globalCache.users[userID].lastValue = value
		if key != "" {
			globalCache.users[userID].keyHistory = append(globalCache.users[userID].keyHistory, key)
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
			if err := cluster.Put(name, key, value, userID); err != nil {
				resp := struct {
					Message string
					Result  string
				}{
					boldHTMLMsg(fmt.Sprintf("[PUT] error %v (key: %q / value: %q)", err, key, value)),
					fmt.Sprintf("<b>[PUT] error %v (key: %q)</b>", err, key),
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
				rs := fmt.Sprintf("success! %q : %q (at %s PST)", keyT, valT, nowPST().String()[:19])
				resp := struct {
					Message string
					Result  string
				}{
					boldHTMLMsg("[PUT] success!"),
					"<b>[PUT]</b> " + rs,
				}
				if err = json.NewEncoder(w).Encode(resp); err != nil {
					return err
				}
			}

		case "GET":
			if vs, err := cluster.Get(name, key, userID); err != nil {
				resp := struct {
					Message string
					Result  string
				}{
					boldHTMLMsg(fmt.Sprintf("[GET] error %v (key: %q)", err, key)),
					fmt.Sprintf("<b>[GET] error %v (key: %q)</b>", err, key),
				}
				if err = json.NewEncoder(w).Encode(resp); err != nil {
					return err
				}
			} else {
				rs := fmt.Sprintf("<b>[GET]</b> %#q (key: %q)", vs, key)
				if len(vs) == 0 {
					rs = fmt.Sprintf("<b>[GET]</b> not exist (key: %q)", key)
				}
				resp := struct {
					Message string
					Result  string
				}{
					boldHTMLMsg("[GET] success!"),
					rs,
				}
				if err = json.NewEncoder(w).Encode(resp); err != nil {
					return err
				}
			}

		case "DELETE":
			if err := cluster.Delete(name, key, userID); err != nil {
				resp := struct {
					Message string
					Result  string
				}{
					boldHTMLMsg(fmt.Sprintf("[DELETE] error %v (key: %q)", err, key)),
					fmt.Sprintf("<b>[DELETE] error %v (key: %q)</b>", err, key),
				}
				if err = json.NewEncoder(w).Encode(resp); err != nil {
					return err
				}
			} else {
				resp := struct {
					Message string
					Result  string
				}{
					boldHTMLMsg("[DELETE] success!"),
					fmt.Sprintf("<b>[DELETE]</b> successfully deleted %q", key),
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
		cluster := globalCache.cluster
		globalCache.mu.Unlock()

		name := urlToName(req.URL.String())
		if err := cluster.Terminate(name); err != nil {
			fmt.Fprintln(w, boldHTMLMsg(fmt.Sprintf("error (%v)", err)))
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
		cluster := globalCache.cluster
		globalCache.mu.Unlock()

		name := urlToName(req.URL.String())
		if err := cluster.Restart(name); err != nil {
			fmt.Fprintln(w, boldHTMLMsg(fmt.Sprintf("error (%v)", err)))
			return err
		}
		fmt.Fprintln(w, boldHTMLMsg(fmt.Sprintf("Restart %s request successfully requested", name)))

	default:
		http.Error(w, "Method Not Allowed", 405)
	}

	return nil
}
