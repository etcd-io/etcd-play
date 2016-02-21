package proc

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
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

	w io.Writer

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

func (nd *NodeTerminal) GetGRPCAddr() string {
	return nd.Flags.ExperimentalgRPCAddr
}

func (nd *NodeTerminal) GetListenClientURLs() []string {
	es := []string{}
	for k := range nd.Flags.ListenClientURLs {
		es = append(es, k)
	}
	sort.Strings(es)
	return es
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
	defer func() {
		if err := recover(); err != nil {
			fmt.Fprintf(nd, "Restart %s: panic (%v)\n", nd.Flags.Name, err)
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
