package commands

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"golang.org/x/net/context"
)

var (
	WebCommand = &cobra.Command{
		Use:   "web",
		Short: "web plays etcd in web browser.",
		Run:   WebCommandFunc,
	}
)

func init() {
	WebCommand.PersistentFlags().StringVarP(&globalWebFlags.EtcdBinary, "etcd-binary", "b", filepath.Join(os.Getenv("GOPATH"), "bin/etcd"), "path of executable etcd binary")
	WebCommand.PersistentFlags().IntVar(&globalWebFlags.ClusterSize, "cluster-size", 5, "size of cluster to create")

	WebCommand.PersistentFlags().BoolVar(&globalWebFlags.LinuxAutoPort, "linux-auto-port", strings.Contains(runtime.GOOS, "linux"), "(only linux supported) 'true' to automate port findings")
	WebCommand.PersistentFlags().DurationVar(&globalWebFlags.LinuxIntervalPortRefresh, "linux-port-refresh", 10*time.Second, "(only linux supported) interval to refresh free ports")

	WebCommand.PersistentFlags().BoolVar(&globalWebFlags.KeepAlive, "keep-alive", false, "'true' to run demo without auto-termination (this overwrites cluster-timeout)")
	WebCommand.PersistentFlags().DurationVar(&globalWebFlags.ClusterTimeout, "cluster-timeout", 5*time.Minute, "after timeout, etcd shuts down the cluster")

	WebCommand.PersistentFlags().IntVar(&globalWebFlags.StressNumber, "stress-number", 10, "size of stress requests")

	WebCommand.PersistentFlags().StringVarP(&globalWebFlags.PlayWebPort, "port", "p", ":8000", "port to serve the play web interface")
	WebCommand.PersistentFlags().BoolVar(&globalWebFlags.Production, "production", false, "'true' when deploying as a web server in production")

	WebCommand.PersistentFlags().BoolVar(&globalWebFlags.IsRemote, "remote", false, "'true' when agents are deployed remotely")
	WebCommand.PersistentFlags().StringSliceVar(&globalWebFlags.AgentEndpoints, "agent-endpoints", []string{"localhost:9027"}, "list of remote agent endpoints")
}

func WebCommandFunc(cmd *cobra.Command, args []string) {
	if globalWebFlags.IsRemote {
		if globalWebFlags.ClusterSize != len(globalWebFlags.AgentEndpoints) {
			fmt.Fprintf(os.Stdout, "[etcd-play error] cluster-size and agent-endpoints must be the same size (%d != %d)\n", globalWebFlags.ClusterSize, len(globalWebFlags.AgentEndpoints))
			os.Exit(0)
		}
	}

	webInit()

	rootContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mainRouter := http.NewServeMux()

	// mainRouter.Handle("/", http.FileServer(http.Dir("./frontend")))
	staticHandler := staticLocalHandler
	if globalWebFlags.IsRemote {
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

	fmt.Fprintln(os.Stdout, "Serving http://localhost"+globalWebFlags.PlayWebPort)
	if err := http.ListenAndServe(globalWebFlags.PlayWebPort, mainRouter); err != nil {
		fmt.Fprintln(os.Stdout, "[etcd-play error]", err)
		os.Exit(0)
	}
}
