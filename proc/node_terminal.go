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
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"sync"
	"syscall"

	"github.com/fatih/color"
)

// NodeTerminal represents an etcd node for terminal.
type NodeTerminal struct {
	// inherit this from Cluster
	pmu                *sync.Mutex
	pmaxProcNameLength *int
	colorIdx           int

	disableLiveLog bool
	w              io.Writer

	ProgramPath string
	Flags       *Flags

	cmd *exec.Cmd
	PID int

	active bool
}

func (nd *NodeTerminal) Write(p []byte) (int, error) {
	buf := bytes.NewBuffer(p)
	wrote := 0
	for {
		line, err := buf.ReadBytes('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return wrote, err
		}
		if len(line) > 1 {
			format := fmt.Sprintf("%%%ds | ", *(nd.pmaxProcNameLength))
			nd.pmu.Lock()
			color.Set(colorsTerminal[nd.colorIdx])
			fmt.Fprintf(nd.w, format, nd.Flags.Name)
			color.Unset()
			fmt.Fprint(nd.w, string(line))
			nd.pmu.Unlock()
			wrote += len(line)
		}
	}

	if len(p) > 0 && p[len(p)-1] != '\n' {
		nd.pmu.Lock()
		fmt.Fprintln(nd.w)
		nd.pmu.Unlock()
	}
	return len(p), nil
}

func (nd *NodeTerminal) Endpoint() string {
	return nd.Flags.ExperimentalgRPCAddr
}

func (nd *NodeTerminal) StatusEndpoint() string {
	es := ""
	for k := range nd.Flags.ListenClientURLs {
		es = k
		break
	}
	return es // TODO: deprecate this v2 endpoint
}

func (nd *NodeTerminal) IsActive() bool {
	nd.pmu.Lock()
	active := nd.active
	nd.pmu.Unlock()
	return active
}

func (nd *NodeTerminal) Start() error {
	defer func() {
		if err := recover(); err != nil {
			fmt.Fprintf(nd, "Start %s: panic (%v)\n", nd.Flags.Name, err)
		}
	}()
	nd.pmu.Lock()
	active := nd.active
	nd.pmu.Unlock()
	if active {
		return fmt.Errorf("%s is already running", nd.Flags.Name)
	}

	shell := os.Getenv("SHELL")
	if len(shell) == 0 {
		shell = "sh"
	}
	nd.pmu.Lock()

	flagString, err := nd.Flags.String()
	if err != nil {
		return err
	}
	args := []string{shell, "-c", nd.ProgramPath + " " + flagString}

	// flagSlice, err := nd.Flags.StringSlice()
	// if err != nil {
	// 	return err
	// }
	// args := append([]string{nd.ProgramPath}, flagSlice...)

	nd.pmu.Unlock()

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = nil
	cmd.Stdout = nd
	cmd.Stderr = nd
	if nd.disableLiveLog {
		cmd.Stdout = ioutil.Discard
		cmd.Stderr = ioutil.Discard
	}

	fmt.Fprintf(nd, "Start %s\n", nd.Flags.Name)
	if err := cmd.Start(); err != nil {
		return err
	}

	nd.pmu.Lock()
	nd.cmd = cmd
	nd.PID = cmd.Process.Pid
	nd.active = true
	nd.pmu.Unlock()

	go func() {
		if err := cmd.Wait(); err != nil {
			fmt.Fprintf(nd, "Start(%s) cmd.Wait returned %v\n", nd.Flags.Name, err)
			return
		}
		fmt.Fprintf(nd, "Exiting %s\n", nd.Flags.Name)
	}()
	return nil
}

func (nd *NodeTerminal) Restart() error {
	nd.pmu.Lock()
	active := nd.active
	nd.pmu.Unlock()
	if active {
		return fmt.Errorf("%s is already running", nd.Flags.Name)
	}
	defer func() {
		if err := recover(); err != nil {
			fmt.Fprintf(nd, "Restart %s: panic (%v)\n", nd.Flags.Name, err)
		}
	}()

	shell := os.Getenv("SHELL")
	if len(shell) == 0 {
		shell = "sh"
	}
	nd.pmu.Lock()
	nd.Flags.InitialClusterState = "existing"
	flagString, err := nd.Flags.String()
	if err != nil {
		return err
	}
	args := []string{shell, "-c", nd.ProgramPath + " " + flagString}
	nd.pmu.Unlock()

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = nil
	cmd.Stdout = nd
	cmd.Stderr = nd

	fmt.Fprintf(nd, "Restart %s\n", nd.Flags.Name)
	if err := cmd.Start(); err != nil {
		return err
	}

	nd.pmu.Lock()
	nd.cmd = cmd
	nd.PID = cmd.Process.Pid
	nd.active = true
	nd.pmu.Unlock()

	go func() {
		if err := cmd.Wait(); err != nil {
			fmt.Fprintf(nd, "Restart(%s) cmd.Wait returned %v\n", nd.Flags.Name, err)
			return
		}
		fmt.Fprintf(nd, "Exiting %s\n", nd.Flags.Name)
	}()
	return nil
}

func (nd *NodeTerminal) Terminate() error {
	defer func() {
		if err := recover(); err != nil {
			fmt.Fprintf(nd, "Terminate %s: panic (%v)\n", nd.Flags.Name, err)
		}
	}()
	nd.pmu.Lock()
	active := nd.active
	nd.pmu.Unlock()
	if !active {
		return fmt.Errorf("%s is already terminated", nd.Flags.Name)
	}

	fmt.Fprintf(nd, "Terminate %s\n", nd.Flags.Name)
	if err := syscall.Kill(nd.PID, syscall.SIGTERM); err != nil {
		return err
	}
	if err := syscall.Kill(nd.PID, syscall.SIGKILL); err != nil {
		return err
	}

	nd.pmu.Lock()
	nd.active = false
	nd.pmu.Unlock()

	return nil
}

func (nd *NodeTerminal) Clean() error {
	defer func() {
		if err := recover(); err != nil {
			fmt.Fprintf(nd, "Clean %s: panic (%v)\n", nd.Flags.Name, err)
		}
	}()
	nd.pmu.Lock()
	active := nd.active
	nd.pmu.Unlock()
	if active {
		return fmt.Errorf("%s is already running", nd.Flags.Name)
	}

	fmt.Fprintf(nd, "Clean %s (%s)\n", nd.Flags.Name, nd.Flags.DataDir)
	if err := os.RemoveAll(nd.Flags.DataDir); err != nil {
		return err
	}
	return nil
}
