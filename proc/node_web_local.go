package proc

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
)

// NodeWebLocal represents an etcd node in local web host.
type NodeWebLocal struct {
	pmu                *sync.Mutex // inherit from Cluster
	pmaxProcNameLength *int
	colorIdx           int

	sharedStream chan string // inherit from Cluster (no need pointer)

	ProgramPath string
	Flags       *Flags

	cmd *exec.Cmd
	PID int

	active bool
}

func (nd *NodeWebLocal) Write(p []byte) (int, error) {
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
			format = fmt.Sprintf(`<b><font color="%s">`, colorsToHTML[colorsTerminal[nd.colorIdx]]) + format + "</font>" + "%s</b>"
			nd.sharedStream <- fmt.Sprintf(format, nd.Flags.Name, line)
			wrote += len(line)
		}
	}

	return len(p), nil
}

func (nd *NodeWebLocal) Endpoint() string {
	return nd.Flags.ExperimentalgRPCAddr
}

func (nd *NodeWebLocal) StatusEndpoint() string {
	es := ""
	for k := range nd.Flags.ListenClientURLs {
		es = k
		break
	}
	return es // TODO: deprecate this v2 endpoint
}

func (nd *NodeWebLocal) IsActive() bool {
	nd.pmu.Lock()
	active := nd.active
	nd.pmu.Unlock()
	return active
}

func (nd *NodeWebLocal) Start() error {
	defer func() {
		if err := recover(); err != nil {
			nd.sharedStream <- fmt.Sprintf("Start %s: panic (%v)\n", nd.Flags.Name, err)
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
	nd.pmu.Unlock()

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = nil
	cmd.Stdout = nd
	cmd.Stderr = nd

	nd.sharedStream <- fmt.Sprintf("Start %s\n", nd.Flags.Name)
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
			nd.sharedStream <- fmt.Sprintf("Start(%s) cmd.Wait returned %v\n", nd.Flags.Name, err)
			return
		}
		nd.sharedStream <- fmt.Sprintf("Exiting %s\n", nd.Flags.Name)
	}()
	return nil
}

func (nd *NodeWebLocal) Restart() error {
	defer func() {
		if err := recover(); err != nil {
			nd.sharedStream <- fmt.Sprintf("Restart %s: panic (%v)\n", nd.Flags.Name, err)
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

	nd.sharedStream <- fmt.Sprintf("Restart %s\n", nd.Flags.Name)
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
			nd.sharedStream <- fmt.Sprintf("Restart(%s) cmd.Wait returned %v\n", nd.Flags.Name, err)
			return
		}
		nd.sharedStream <- fmt.Sprintf("Exiting %s\n", nd.Flags.Name)
	}()
	return nil
}

func (nd *NodeWebLocal) Terminate() error {
	defer func() {
		if err := recover(); err != nil {
			nd.sharedStream <- fmt.Sprintf("Terminate %s: panic (%v)\n", nd.Flags.Name, err)
		}
	}()
	nd.pmu.Lock()
	active := nd.active
	nd.pmu.Unlock()
	if !active {
		return fmt.Errorf("%s is already terminated", nd.Flags.Name)
	}

	nd.sharedStream <- fmt.Sprintf("Terminate %s\n", nd.Flags.Name)
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

func (nd *NodeWebLocal) Clean() error {
	defer func() {
		if err := recover(); err != nil {
			nd.sharedStream <- fmt.Sprintf("Clean %s: panic (%v)\n", nd.Flags.Name, err)
		}
	}()
	nd.pmu.Lock()
	active := nd.active
	nd.pmu.Unlock()
	if active {
		return fmt.Errorf("%s is already running", nd.Flags.Name)
	}

	nd.sharedStream <- fmt.Sprintf("Clean %s (%s)\n", nd.Flags.Name, nd.Flags.DataDir)
	if err := os.RemoveAll(nd.Flags.DataDir); err != nil {
		return err
	}
	return nil
}
