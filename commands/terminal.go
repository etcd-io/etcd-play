package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/coreos/etcd-play/proc"
	"github.com/spf13/cobra"
)

type TerminalFlags struct {
	EtcdBinary  string
	ClusterSize int

	LinuxAutoPort            bool
	LinuxIntervalPortRefresh time.Duration

	KeepAlive      bool
	ClusterTimeout time.Duration
	Pause          time.Duration

	StressNumber int

	// TODO
	// IsClientTLS bool
	// IsPeerTLS   bool
}

var (
	TerminalCommand = &cobra.Command{
		Use:   "terminal",
		Short: "terminal runs etcd in terminal.",
		Run:   TerminalCommandFunc,
	}
	globalTerminalFlags = TerminalFlags{}
)

func init() {
	cobra.EnablePrefixMatching = true
}

func init() {
	TerminalCommand.PersistentFlags().StringVarP(&globalTerminalFlags.EtcdBinary, "etcd-binary", "b", filepath.Join(os.Getenv("GOPATH"), "bin/etcd"), "path of executable etcd binary")
	TerminalCommand.PersistentFlags().IntVarP(&globalTerminalFlags.ClusterSize, "cluster-size", "n", 5, "size of cluster to create")

	TerminalCommand.PersistentFlags().BoolVar(&globalTerminalFlags.LinuxAutoPort, "linux-auto-port", strings.Contains(runtime.GOOS, "linux"), "(only linux supported) 'true' to automate port findings")
	TerminalCommand.PersistentFlags().DurationVar(&globalTerminalFlags.LinuxIntervalPortRefresh, "linux-port-refresh", 10*time.Second, "(only linux supported) interval to refresh free ports")

	TerminalCommand.PersistentFlags().BoolVar(&globalTerminalFlags.KeepAlive, "keep-alive", false, "'true' to run demo without auto-termination (this overwrites cluster-timeout)")
	TerminalCommand.PersistentFlags().DurationVar(&globalTerminalFlags.ClusterTimeout, "cluster-timeout", 5*time.Minute, "after timeout, etcd shuts down the cluster")
	TerminalCommand.PersistentFlags().DurationVar(&globalTerminalFlags.Pause, "pause", 10*time.Second, "duration to pause between demo operations")

	TerminalCommand.PersistentFlags().IntVar(&globalTerminalFlags.StressNumber, "stress-number", 10, "size of stress requests")
}

func TerminalCommandFunc(cmd *cobra.Command, args []string) {
	if globalTerminalFlags.LinuxAutoPort {
		globalPorts.Refresh()
		go func() {
			for {
				select {
				case <-time.After(globalTerminalFlags.LinuxIntervalPortRefresh):
					globalPorts.Refresh()
				}
			}
		}()
	}

	defer func() {
		if err := recover(); err != nil {
			fmt.Fprintln(os.Stdout, "[RunTerminal panic]", err)
			os.Exit(0)
		}
	}()

	fs := make([]*proc.Flags, globalTerminalFlags.ClusterSize)
	for i := range fs {
		df, err := proc.GenerateFlags(fmt.Sprintf("etcd%d", i+1), "localhost", false, globalPorts)
		if err != nil {
			fmt.Fprintln(os.Stdout, "exiting with:", err)
			return
		}
		fs[i] = df
	}

	c, err := proc.NewCluster(proc.Terminal, nil, globalTerminalFlags.EtcdBinary, fs...)
	if err != nil {
		fmt.Fprintln(os.Stdout, "exiting with:", err)
		return
	}
	defer c.Shutdown()

	fmt.Fprintf(os.Stdout, "\n####### Bootstrap %d nodes\n", globalTerminalFlags.ClusterSize)
	clusterDone := make(chan struct{})
	go func() {
		defer func() {
			clusterDone <- struct{}{}
		}()
		if err := c.Bootstrap(); err != nil {
			fmt.Fprintln(os.Stdout, "exiting with:", err)
			return
		}
	}()

	operationDone := make(chan struct{})
	go func() {
		if !globalTerminalFlags.KeepAlive {
			defer func() {
				operationDone <- struct{}{}
			}()
		}

		time.Sleep(globalTerminalFlags.Pause)
		fmt.Fprintf(os.Stdout, "\n####### Terminate Leader\n")
		leaderName, err := c.Leader()
		if err != nil {
			fmt.Fprintln(os.Stdout, "exiting with:", err)
			return
		}
		if err := c.Terminate(leaderName); err != nil {
			fmt.Fprintln(os.Stdout, "exiting with:", err)
			return
		}

		time.Sleep(globalTerminalFlags.Pause)
		fmt.Fprintf(os.Stdout, "\n####### Status\n")
		vm, err := c.Status()
		if err != nil {
			fmt.Fprintln(os.Stdout, "Status error:", err)
		}
		fmt.Fprintf(os.Stdout, "\nName to ServerStus:\n")
		for name, st := range vm {
			fmt.Fprintf(os.Stdout, "%s = ID: %16s | Endpoint: %5s | State: %13s | Number of Keys: %3d | Hash: %7d\n",
				name, st.ID, st.Endpoint, st.State, st.NumberOfKeys, st.Hash)
		}
		fmt.Println()

		// Stress here to trigger log compaction
		// (make terminated node fall behind)

		time.Sleep(globalTerminalFlags.Pause)
		fmt.Fprintf(os.Stdout, "\n####### Restart Leader\n")
		if err := c.Restart(leaderName); err != nil {
			fmt.Fprintln(os.Stdout, "exiting with:", err)
			return
		}

		key, val := "sample_key", "sample_value"
		time.Sleep(globalTerminalFlags.Pause)
		fmt.Fprintf(os.Stdout, "\n####### Put %q\n", key)
		if err := c.Put(leaderName, key, val); err != nil {
			fmt.Fprintln(os.Stdout, "exiting with:", err)
			return
		}
		if err := c.Put("", key, val); err != nil {
			fmt.Fprintln(os.Stdout, "exiting with:", err)
			return
		}

		time.Sleep(globalTerminalFlags.Pause)
		fmt.Fprintf(os.Stdout, "\n####### Get %q\n", key)
		if _, err := c.Get(leaderName, key); err != nil {
			fmt.Fprintln(os.Stdout, "exiting with:", err)
			return
		}
		if _, err := c.Get("", key); err != nil {
			fmt.Fprintln(os.Stdout, "exiting with:", err)
			return
		}

		time.Sleep(globalTerminalFlags.Pause)
		fmt.Fprintf(os.Stdout, "\n####### Delete %q\n", key)
		if err := c.Delete("", key); err != nil {
			fmt.Fprintln(os.Stdout, "exiting with:", err)
			return
		}

		time.Sleep(globalTerminalFlags.Pause)
		fmt.Fprintf(os.Stdout, "\n####### Get %q\n", key)
		if _, err := c.Get(leaderName, key); err != nil {
			fmt.Fprintln(os.Stdout, "exiting with:", err)
			return
		}
		if _, err := c.Get("", key); err != nil {
			fmt.Fprintln(os.Stdout, "exiting with:", err)
			return
		}

		time.Sleep(globalTerminalFlags.Pause)
		fmt.Fprintf(os.Stdout, "\n####### Stress\n")
		if err := c.Stress(leaderName, globalTerminalFlags.StressNumber); err != nil {
			fmt.Fprintln(os.Stdout, "exiting with:", err)
			return
		}

		time.Sleep(globalTerminalFlags.Pause)
		fmt.Fprintf(os.Stdout, "\n####### WatchPut\n")
		watcherN := globalTerminalFlags.StressNumber
		if err := c.WatchPut(leaderName, watcherN); err != nil {
			fmt.Fprintln(os.Stdout, "exiting with:", err)
			return
		}

		time.Sleep(globalTerminalFlags.Pause)
		fmt.Fprintf(os.Stdout, "\n####### Status\n")
		vm, err = c.Status()
		if err != nil {
			fmt.Fprintln(os.Stdout, "Status error:", err)
		}
		fmt.Fprintf(os.Stdout, "Name to ServerStus:\n")
		for name, st := range vm {
			fmt.Fprintf(os.Stdout, "%s = ID: %16s | Endpoint: %5s | State: %13s | Number of Keys: %3d | Hash: %7d\n",
				name, st.ID, st.Endpoint, st.State, st.NumberOfKeys, st.Hash)
		}
		fmt.Println()
	}()

	select {
	case <-clusterDone:
		fmt.Fprintln(os.Stdout, "[RunTerminal END] etcd cluster terminated!")
		return
	case <-operationDone:
		fmt.Fprintln(os.Stdout, "[RunTerminal END] operation terminated!")
		return
	case <-time.After(globalTerminalFlags.ClusterTimeout):
		fmt.Fprintln(os.Stdout, "[RunTerminal END] timed out!")
		return
	}
}
