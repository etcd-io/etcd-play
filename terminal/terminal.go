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

package terminal

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/coreos/etcd-play/proc"
	"github.com/gyuho/psn/ss"
	"github.com/spf13/cobra"
)

type Flags struct {
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
	Command = &cobra.Command{
		Use:   "terminal",
		Short: "terminal runs etcd in terminal.",
		Run:   CommandFunc,
	}
	globalFlags = Flags{}
	globalPorts = ss.NewPorts()
)

func init() {
	cobra.EnablePrefixMatching = true
}

func init() {
	Command.PersistentFlags().StringVarP(&globalFlags.EtcdBinary, "etcd-binary", "b", filepath.Join(os.Getenv("GOPATH"), "bin/etcd"), "path of executable etcd binary")
	Command.PersistentFlags().IntVarP(&globalFlags.ClusterSize, "cluster-size", "n", 5, "size of cluster to create")

	Command.PersistentFlags().BoolVar(&globalFlags.LinuxAutoPort, "linux-auto-port", strings.Contains(runtime.GOOS, "linux"), "(only linux supported) 'true' to automate port findings")
	Command.PersistentFlags().DurationVar(&globalFlags.LinuxIntervalPortRefresh, "linux-port-refresh", 10*time.Second, "(only linux supported) interval to refresh free ports")

	Command.PersistentFlags().BoolVar(&globalFlags.KeepAlive, "keep-alive", false, "'true' to run demo without auto-termination (this overwrites cluster-timeout)")
	Command.PersistentFlags().DurationVar(&globalFlags.ClusterTimeout, "cluster-timeout", 5*time.Minute, "after timeout, etcd shuts down the cluster")
	Command.PersistentFlags().DurationVar(&globalFlags.Pause, "pause", 10*time.Second, "duration to pause between demo operations")

	Command.PersistentFlags().IntVar(&globalFlags.StressNumber, "stress-number", 10, "size of stress requests")
}

func CommandFunc(cmd *cobra.Command, args []string) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Fprintln(os.Stdout, "[terminal]", err)
			os.Exit(0)
		}
	}()

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

	fs := make([]*proc.Flags, globalFlags.ClusterSize)
	for i := range fs {
		df, err := proc.GenerateFlags(fmt.Sprintf("etcd%d", i+1), "localhost", false, globalPorts)
		if err != nil {
			fmt.Fprintln(os.Stdout, "exiting with:", err)
			return
		}
		fs[i] = df
	}

	disableLiveLog := false
	agentEndpoints := []string{}
	c, err := proc.NewCluster(proc.Terminal, disableLiveLog, 0, agentEndpoints, globalFlags.EtcdBinary, fs...)
	if err != nil {
		fmt.Fprintln(os.Stdout, "exiting with:", err)
		return
	}
	defer c.Shutdown()

	fmt.Fprintf(os.Stdout, "\n####### Bootstrap %d nodes\n", globalFlags.ClusterSize)
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
		if !globalFlags.KeepAlive {
			defer func() {
				operationDone <- struct{}{}
			}()
		}

		time.Sleep(globalFlags.Pause)
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

		time.Sleep(globalFlags.Pause)
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

		time.Sleep(globalFlags.Pause)
		fmt.Fprintf(os.Stdout, "\n####### Restart Leader\n")
		if err := c.Restart(leaderName); err != nil {
			fmt.Fprintln(os.Stdout, "exiting with:", err)
			return
		}

		key, val := "sample_key", "sample_value"
		time.Sleep(globalFlags.Pause)
		fmt.Fprintf(os.Stdout, "\n####### Put %q\n", key)
		if err := c.Put(leaderName, key, val); err != nil {
			fmt.Fprintln(os.Stdout, "exiting with:", err)
			return
		}
		if err := c.Put("", key, val); err != nil {
			fmt.Fprintln(os.Stdout, "exiting with:", err)
			return
		}

		time.Sleep(globalFlags.Pause)
		fmt.Fprintf(os.Stdout, "\n####### Get %q\n", key)
		if _, err := c.Get(leaderName, key); err != nil {
			fmt.Fprintln(os.Stdout, "exiting with:", err)
			return
		}
		if _, err := c.Get("", key); err != nil {
			fmt.Fprintln(os.Stdout, "exiting with:", err)
			return
		}

		time.Sleep(globalFlags.Pause)
		fmt.Fprintf(os.Stdout, "\n####### Delete %q\n", key)
		if err := c.Delete("", key); err != nil {
			fmt.Fprintln(os.Stdout, "exiting with:", err)
			return
		}

		time.Sleep(globalFlags.Pause)
		fmt.Fprintf(os.Stdout, "\n####### Get %q\n", key)
		if _, err := c.Get(leaderName, key); err != nil {
			fmt.Fprintln(os.Stdout, "exiting with:", err)
			return
		}
		if _, err := c.Get("", key); err != nil {
			fmt.Fprintln(os.Stdout, "exiting with:", err)
			return
		}

		time.Sleep(globalFlags.Pause)
		fmt.Fprintf(os.Stdout, "\n####### Stress\n")
		if err := c.Stress(leaderName, globalFlags.StressNumber); err != nil {
			fmt.Fprintln(os.Stdout, "exiting with:", err)
			return
		}

		time.Sleep(globalFlags.Pause)
		fmt.Fprintf(os.Stdout, "\n####### WatchPut\n")
		watcherN := globalFlags.StressNumber
		if err := c.WatchPut(leaderName, watcherN); err != nil {
			fmt.Fprintln(os.Stdout, "exiting with:", err)
			return
		}

		time.Sleep(globalFlags.Pause)
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
	case <-time.After(globalFlags.ClusterTimeout):
		fmt.Fprintln(os.Stdout, "[RunTerminal END] timed out!")
		return
	}
}
