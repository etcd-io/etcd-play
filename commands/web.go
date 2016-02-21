package commands

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coreos/etcd-play/proc"
	"github.com/gorilla/websocket"
	"github.com/gyuho/psn/ss"
	"golang.org/x/net/context"
)

type (
	key int

	userData struct {
		upgrader *websocket.Upgrader

		startTime time.Time // time it clicked 'Play etcd'

		// rate limit
		lastRequestTime time.Time
		requestCount    int

		online bool

		selectedNodeName  string
		selectedOperation string

		lastKey   string
		lastValue string

		keyHistory []string
	}

	sharedData struct {
		mu      sync.Mutex
		cluster proc.Cluster
		users   map[string]*userData
	}

	WebFlags struct {
		EtcdBinary  string
		ClusterSize int

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
	globalPorts                = ss.NewPorts()
	globalCache    *sharedData = nil
	globalWebFlags             = WebFlags{}
)

// webInit must be called at the beginning of 'local' and 'remote' commands.
func webInit() {
	if globalWebFlags.LinuxAutoPort {
		globalPorts.Refresh()
		go func() {
			for {
				select {
				case <-time.After(globalWebFlags.LinuxIntervalPortRefresh):
					globalPorts.Refresh()
				}
			}
		}()
	}

	data := sharedData{
		cluster: nil,
		users:   make(map[string]*userData),
	}
	globalCache = &data

	globalCache.mu.Lock()
	if globalCache.users == nil {
		globalCache.users = make(map[string]*userData)
	}
	globalCache.mu.Unlock()

	// clean up users that started more than 2 hours ago
	go func() {
		copied := make(map[string]time.Time)
		for {
			globalCache.mu.Lock()
			for k, v := range globalCache.users {
				copied[k] = v.startTime
			}
			globalCache.mu.Unlock()

			now := time.Now()
			usersToDelete := make(map[string]struct{})
			for k, v := range copied {
				sub := now.Sub(v)
				if sub > 2*time.Hour {
					usersToDelete[k] = struct{}{}
				}
			}

			globalCache.mu.Lock()
			for user := range usersToDelete {
				_, ok := globalCache.users[user]
				if ok {
					delete(globalCache.users, user)
				}
			}
			globalCache.mu.Unlock()

			copied = make(map[string]time.Time)
			time.Sleep(2 * time.Hour)
		}
	}()

	// clean up users that just left the browser
	go func() {
		for {
			globalCache.mu.Lock()
			for k, v := range globalCache.users {
				if !v.online {
					delete(globalCache.users, k)
				}
			}
			globalCache.mu.Unlock()
			time.Sleep(10 * time.Minute)
		}
	}()
}

// checkCluster returns the cluster if the cluster is active.
func (s *sharedData) clusterActive() bool {
	s.mu.Lock()
	clu := s.cluster
	s.mu.Unlock()
	return clu != nil
}

func (s *sharedData) okToRequest(userID string) bool {
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

func withCache(h ContextHandler) ContextHandler {
	return ContextHandlerFunc(func(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
		userID := getUserID(req)
		ctx = context.WithValue(ctx, userKey, &userID)

		globalCache.mu.Lock()
		if _, ok := globalCache.users[userID]; !ok {
			globalCache.users[userID] = &userData{
				upgrader:  &websocket.Upgrader{},
				startTime: time.Now(),
				online:    true,
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

// wsHandler monitors user activities and notifies when a user leaves the web pages.
func wsHandler(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
	user := ctx.Value(userKey).(*string)
	userID := *user

	globalCache.mu.Lock()
	upgrader := globalCache.users[userID].upgrader
	globalCache.mu.Unlock()

	c, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		globalCache.mu.Lock()
		globalCache.users[userID].online = false
		globalCache.mu.Unlock()
		return err
	}
	defer c.Close()

	for {
		mt, message, err := c.ReadMessage()
		if err != nil {
			globalCache.mu.Lock()
			globalCache.users[userID].online = false
			globalCache.mu.Unlock()
			return err
		}
		if err := c.WriteMessage(mt, message); err != nil {
			globalCache.mu.Lock()
			globalCache.users[userID].online = false
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
	if globalWebFlags.IsRemote {
		nodeType = proc.WebRemote
	}

	switch req.Method {
	case "GET":
		wport := globalWebFlags.PlayWebPort
		if globalWebFlags.Production {
			wport = ":80"
		}
		resp := struct {
			Message     string
			PlayWebPort string
		}{
			"",
			wport,
		}
		globalCache.mu.Lock()
		globalCache.users[userID].online = true
		globalCache.mu.Unlock()

		if globalCache.clusterActive() {
			resp.Message = boldHTMLMsg("Cluster is already running...")
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				return err
			}
			return nil
		}

		done, errc := make(chan struct{}), make(chan error)
		go startCluster(nodeType, globalWebFlags.ClusterSize, globalWebFlags.AgentEndpoints, userID, done, errc)
		select {
		case <-done:
			resp.Message = boldHTMLMsg("Start cluster successfully requested!!!")
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				return err
			}
		case err := <-errc:
			resp.Message = boldHTMLMsg(fmt.Sprintf("Start cluster failed (%v)", err))
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				return err
			}
		}

	default:
		http.Error(w, "Method Not Allowed", 405)
	}

	return nil
}

func startCluster(nodeType proc.NodeType, clusterSize int, agentEndpoints []string, userID string, done chan struct{}, errc chan error) {
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

	c, err := proc.NewCluster(nodeType, agentEndpoints, globalWebFlags.EtcdBinary, fs...)
	if err != nil {
		errc <- err
		return
	}

	globalCache.mu.Lock()
	globalCache.cluster = c
	globalCache.mu.Unlock()

	// this does not run with the program exits with os.Exit(0)
	defer func() {
		if !globalWebFlags.KeepAlive {
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
		c.Stream(userID) <- boldHTMLMsg(fmt.Sprintf("Starting %d nodes", globalWebFlags.ClusterSize))
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

	case <-time.After(globalWebFlags.ClusterTimeout):
		if !globalWebFlags.KeepAlive {
			c.Stream(userID) <- boldHTMLMsg(fmt.Sprintf("Cluster time out (%v)! Please restart the cluster.", globalWebFlags.ClusterTimeout))
		}
	}
}

func serverStatusHandler(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
	switch req.Method {
	case "GET":
		if !globalCache.clusterActive() {
			return nil
		}

		globalCache.mu.Lock()
		cluster := globalCache.cluster
		globalCache.mu.Unlock()

		nameToStatus, _ := cluster.Status()
		etcd1_ID, etcd1_Endpoint, etcd1_State := "unknown", "unknown", ""
		etcd1_NumberOfKeys, etcd1_Hash := 0, 0
		if v, ok := nameToStatus["etcd1"]; ok {
			etcd1_ID = v.ID
			etcd1_Endpoint = v.Endpoint
			etcd1_State = v.State
			etcd1_NumberOfKeys = v.NumberOfKeys
			etcd1_Hash = v.Hash
		}
		etcd2_ID, etcd2_Endpoint, etcd2_State := "unknown", "unknown", ""
		etcd2_NumberOfKeys, etcd2_Hash := 0, 0
		if v, ok := nameToStatus["etcd2"]; ok {
			etcd2_ID = v.ID
			etcd2_Endpoint = v.Endpoint
			etcd2_State = v.State
			etcd2_NumberOfKeys = v.NumberOfKeys
			etcd2_Hash = v.Hash
		}
		etcd3_ID, etcd3_Endpoint, etcd3_State := "unknown", "unknown", ""
		etcd3_NumberOfKeys, etcd3_Hash := 0, 0
		if v, ok := nameToStatus["etcd3"]; ok {
			etcd3_ID = v.ID
			etcd3_Endpoint = v.Endpoint
			etcd3_State = v.State
			etcd3_NumberOfKeys = v.NumberOfKeys
			etcd3_Hash = v.Hash
		}
		etcd4_ID, etcd4_Endpoint, etcd4_State := "unknown", "unknown", ""
		etcd4_NumberOfKeys, etcd4_Hash := 0, 0
		if v, ok := nameToStatus["etcd4"]; ok {
			etcd4_ID = v.ID
			etcd4_Endpoint = v.Endpoint
			etcd4_State = v.State
			etcd4_NumberOfKeys = v.NumberOfKeys
			etcd4_Hash = v.Hash
		}
		etcd5_ID, etcd5_Endpoint, etcd5_State := "unknown", "unknown", ""
		etcd5_NumberOfKeys, etcd5_Hash := 0, 0
		if v, ok := nameToStatus["etcd5"]; ok {
			etcd5_ID = v.ID
			etcd5_Endpoint = v.Endpoint
			etcd5_State = v.State
			etcd5_NumberOfKeys = v.NumberOfKeys
			etcd5_Hash = v.Hash
		}

		resp := struct {
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
		if len(history) > 7 { // FIFO of at most 5 command histories
			temp := make([]string, 7)
			copy(temp, history[:1])
			copy(temp[1:], history[2:])
			history = temp
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
					boldHTMLMsg(fmt.Sprintf("[PUT] error %v (key: %s / value: %s)", err, key, value)),
					"",
				}
				if err = json.NewEncoder(w).Encode(resp); err != nil {
					return err
				}
			} else {
				resp := struct {
					Message string
					Result  string
				}{
					boldHTMLMsg("[PUT] success!"),
					"",
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
					boldHTMLMsg(fmt.Sprintf("[GET] error %v (key: %s)", err, key)),
					fmt.Sprintf("<b>[GET] error %v (key: %s)</b>", err, key),
				}
				if err = json.NewEncoder(w).Encode(resp); err != nil {
					return err
				}
			} else {
				rs := fmt.Sprintf("<b>[GET]</b> %#q (key: %s)", vs, key)
				if len(vs) == 0 {
					rs = fmt.Sprintf("<b>[GET]</b> not exist (key: %s)", key)
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
					boldHTMLMsg(fmt.Sprintf("[DELETE] error %v (key: %s)", err, key)),
					fmt.Sprintf("<b>[DELETE] error %v (key: %s)</b>", err, key),
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
					fmt.Sprintf("<b>[DELETE]</b> successfully deleted %s", key),
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
