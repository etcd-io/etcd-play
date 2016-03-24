package ss

import (
	"encoding/csv"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/user"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gyuho/psn/table"
	"github.com/olekukonko/tablewriter"
)

type TransportProtocol int

const (
	TCP TransportProtocol = iota
	TCP6
)

var (
	stringToProtocol = map[string]TransportProtocol{
		"tcp":  TCP,
		"tcp6": TCP6,
	}
	protocolToString = map[TransportProtocol]string{
		TCP:  "tcp",
		TCP6: "tcp6",
	}
)

// Process describes OS processes.
type Process struct {
	Protocol string

	Program string
	PID     int

	LocalIP   string
	LocalPort string

	RemoteIP   string
	RemotePort string

	State string

	User user.User
}

// ProcessTableColumns is columns for CSV file.
var ProcessTableColumns = []string{
	"PROTOCOL",
	"PROGRAM",
	"PID",
	"LOCAL_ADDR",
	"REMOTE_ADDR",
	"USER",
}

// String describes Process type.
func (p Process) String() string {
	return fmt.Sprintf("Protocol: '%s' | Program: '%s' | PID: '%d' | LocalAddr: '%s%s' | RemoteAddr: '%s%s' | State: '%s' | User: '%+v'",
		p.Protocol,
		p.Program,
		p.PID,
		p.LocalIP,
		p.LocalPort,
		p.RemoteIP,
		p.RemotePort,
		p.State,
		p.User,
	)
}

// Kill kills all processes in arguments.
func Kill(w io.Writer, ps ...Process) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Fprintln(w, "Kill:", err)
		}
	}()

	pidToKill := make(map[int]string)
	for _, p := range ps {
		pidToKill[p.PID] = p.Program
	}
	if len(pidToKill) == 0 {
		fmt.Fprintln(w, "no PID to kill...")
		return
	}

	pids := []int{}
	for pid := range pidToKill {
		pids = append(pids, pid)
	}
	sort.Ints(pids)

	for _, pid := range pids {
		fmt.Fprintf(w, "\nsyscall.Kill: %s [PID: %d]\n", pidToKill[pid], pid)
		if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
			fmt.Fprintf(w, "syscall.SIGTERM error (%v)\n", err)

			shell := os.Getenv("SHELL")
			if len(shell) == 0 {
				shell = "sh"
			}
			args := []string{shell, "-c", fmt.Sprintf("sudo kill -9 %d", pid)}
			cmd := exec.Command(args[0], args[1:]...)
			cmd.Stdout = w
			cmd.Stderr = w
			fmt.Fprintf(w, "Starting: %q\n", strings.Join(cmd.Args, ""))
			if err := cmd.Start(); err != nil {
				fmt.Fprintf(w, "error when 'sudo kill -9' (%v)\n", err)
			}
			if err := cmd.Wait(); err != nil {
				fmt.Fprintf(w, "Start(%s) cmd.Wait returned %v\n", cmd.Path, err)
			}
			fmt.Fprintf(w, "    Done: %q\n", strings.Join(cmd.Args, ""))
		}
		if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
			fmt.Fprintf(w, "syscall.SIGKILL error (%v)\n", err)

			shell := os.Getenv("SHELL")
			if len(shell) == 0 {
				shell = "sh"
			}
			args := []string{shell, "-c", fmt.Sprintf("sudo kill -9 %d", pid)}
			cmd := exec.Command(args[0], args[1:]...)
			cmd.Stdout = w
			cmd.Stderr = w
			fmt.Fprintf(w, "Starting: %q\n", strings.Join(cmd.Args, ""))
			if err := cmd.Start(); err != nil {
				fmt.Fprintf(w, "error when 'sudo kill -9' (%v)\n", err)
			}
			if err := cmd.Wait(); err != nil {
				fmt.Fprintf(w, "Start(%s) cmd.Wait returned %v\n", cmd.Path, err)
			}
			fmt.Fprintf(w, "    Done: %q\n", strings.Join(cmd.Args, ""))
		}
	}
	fmt.Fprintln(w)
}

// parseLittleEndianIpv4 parses hexadecimal ipv4 IP addresses.
// For example, it converts '0101007F:0035' into string.
// It assumes that the system has little endian order.
func parseLittleEndianIpv4(s string) (string, string, error) {
	arr := strings.Split(s, ":")
	if len(arr) != 2 {
		return "", "", fmt.Errorf("cannot parse ipv4 %s", s)
	}
	if len(arr[0]) != 8 {
		return "", "", fmt.Errorf("cannot parse ipv4 ip %s", arr[0])
	}
	if len(arr[1]) != 4 {
		return "", "", fmt.Errorf("cannot parse ipv4 port %s", arr[1])
	}

	d0, err := strconv.ParseInt(arr[0][6:8], 16, 32)
	if err != nil {
		return "", "", err
	}
	d1, err := strconv.ParseInt(arr[0][4:6], 16, 32)
	if err != nil {
		return "", "", err
	}
	d2, err := strconv.ParseInt(arr[0][2:4], 16, 32)
	if err != nil {
		return "", "", err
	}
	d3, err := strconv.ParseInt(arr[0][0:2], 16, 32)
	if err != nil {
		return "", "", err
	}
	ip := fmt.Sprintf("%d.%d.%d.%d", d0, d1, d2, d3)

	p0, err := strconv.ParseInt(arr[1], 16, 32)
	if err != nil {
		return "", "", err
	}
	port := fmt.Sprintf("%d", p0)

	return ip, ":" + port, nil
}

// parseLittleEndianIpv6 parses hexadecimal ipv6 IP addresses.
// For example, it converts '4506012691A700C165EB1DE1F912918C:8BDA'
// into string.
// It assumes that the system has little endian order.
func parseLittleEndianIpv6(s string) (string, string, error) {
	arr := strings.Split(s, ":")
	if len(arr) != 2 {
		return "", "", fmt.Errorf("cannot parse ipv6 %s", s)
	}
	if len(arr[0]) != 32 {
		return "", "", fmt.Errorf("cannot parse ipv6 ip %s", arr[0])
	}
	if len(arr[1]) != 4 {
		return "", "", fmt.Errorf("cannot parse ipv6 port %s", arr[1])
	}

	// 32 characters, reverse by 2 characters
	ip := ""
	for i := 32; i > 0; i -= 2 {
		ip += fmt.Sprintf("%s", arr[0][i-2:i])
		if (len(ip)+1)%5 == 0 && len(ip) != 39 {
			ip += ":"
		}
	}

	p0, err := strconv.ParseInt(arr[1], 16, 32)
	if err != nil {
		return "", "", err
	}
	port := fmt.Sprintf("%d", p0)

	return ip, ":" + port, nil
}

// List lists all processes. filter is used to return Processes
// that matches the member values in filter struct.
func List(filter *Process, opts ...TransportProtocol) ([]Process, error) {
	rs := []Process{}
	for _, opt := range opts {
		ps, err := list(opt)
		if err != nil {
			return nil, err
		}

		pc, done := make(chan Process), make(chan struct{})
		for _, p := range ps {
			go func(process Process, ft *Process) {
				if !process.Match(ft) {
					done <- struct{}{}
					return
				}
				pc <- process
			}(p, filter)
		}

		cn := 0
		ns := []Process{}
		for cn != len(ps) {
			select {
			case p := <-pc:
				ns = append(ns, p)
				cn++
			case <-done:
				cn++
			}
		}

		close(pc)
		close(done)

		rs = append(rs, ns...)
	}
	return rs, nil
}

// Match returns true if the Process matches the filter.
func (p *Process) Match(filter *Process) bool {
	if p == nil {
		return false
	}
	if filter == nil {
		return true
	}
	if filter.Protocol != "" {
		if p.Protocol != filter.Protocol {
			return false
		}
	}
	// matches the suffix
	if filter.Program != "" {
		if !strings.HasSuffix(p.Program, filter.Program) {
			return false
		}
	}
	if filter.PID != 0 {
		if p.PID != filter.PID {
			return false
		}
	}
	if filter.LocalIP != "" {
		if p.LocalIP != filter.LocalIP {
			return false
		}
	}
	if filter.LocalPort != "" {
		p0 := p.LocalPort
		if !strings.HasPrefix(p0, ":") {
			p0 = ":" + p0
		}
		p1 := filter.LocalPort
		if !strings.HasPrefix(p1, ":") {
			p1 = ":" + p1
		}
		if p0 != p1 {
			return false
		}
	}
	if filter.RemoteIP != "" {
		if p.RemoteIP != filter.RemoteIP {
			return false
		}
	}
	if filter.RemotePort != "" {
		p0 := p.RemotePort
		if !strings.HasPrefix(p0, ":") {
			p0 = ":" + p0
		}
		p1 := filter.RemotePort
		if !strings.HasPrefix(p1, ":") {
			p1 = ":" + p1
		}
		if p0 != p1 {
			return false
		}
	}
	if filter.State != "" {
		if p.State != filter.State {
			return false
		}
	}
	// currently only support user name
	if filter.User.Username != "" {
		if p.User.Username != filter.User.Username {
			return false
		}
	}
	return true
}

// WriteToTable writes slice of Processes to ASCII table.
func WriteToTable(w io.Writer, top int, ps ...Process) {
	tw := tablewriter.NewWriter(w)
	tw.SetHeader(ProcessTableColumns)

	rows := make([][]string, len(ps))
	for i, p := range ps {
		sl := make([]string, len(ProcessTableColumns))
		sl[0] = p.Protocol
		sl[1] = p.Program
		sl[2] = strconv.Itoa(p.PID)
		sl[3] = p.LocalIP + p.LocalPort
		sl[4] = p.RemoteIP + p.RemotePort
		sl[5] = p.User.Name
		rows[i] = sl
	}

	table.By(
		rows,
		table.MakeAscendingFunc(0), // PROTOCOL
		table.MakeAscendingFunc(1), // PROGRAM
		table.MakeAscendingFunc(2), // PID
		table.MakeAscendingFunc(3), // LOCAL_ADDR
		table.MakeAscendingFunc(5), // USER
	).Sort(rows)

	if top != 0 && len(rows) > top {
		rows = rows[:top:top]
	}
	for _, row := range rows {
		tw.Append(row)
	}

	tw.Render()
}

var once sync.Once

// WriteToCSV writes slice of Processes to a csv file.
func WriteToCSV(f *os.File, ps ...Process) error {
	wr := csv.NewWriter(f)

	var werr error
	writeCSVHeader := func() {
		if err := wr.Write(append([]string{"unix_ts"}, ProcessTableColumns...)); err != nil {
			werr = err
		}
	}
	once.Do(writeCSVHeader)
	if werr != nil {
		return werr
	}

	rows := make([][]string, len(ps))
	for i, p := range ps {
		sl := make([]string, len(ProcessTableColumns))
		sl[0] = p.Protocol
		sl[1] = p.Program
		sl[2] = strconv.Itoa(p.PID)
		sl[3] = p.LocalIP + p.LocalPort
		sl[4] = p.RemoteIP + p.RemotePort
		sl[5] = p.User.Name
		rows[i] = sl
	}

	table.By(
		rows,
		table.MakeAscendingFunc(0), // PROTOCOL
		table.MakeAscendingFunc(1), // PROGRAM
		table.MakeAscendingFunc(2), // PID
		table.MakeAscendingFunc(3), // LOCAL_ADDR
		table.MakeAscendingFunc(5), // USER
	).Sort(rows)

	// adding timestamp
	ts := fmt.Sprintf("%d", time.Now().Unix())
	nrows := make([][]string, len(rows))
	for i, row := range rows {
		nrows[i] = append([]string{ts}, row...)
	}
	if err := wr.WriteAll(nrows); err != nil {
		return err
	}

	wr.Flush()
	return wr.Error()
}

// ListPorts lists all ports that are being used.
func ListPorts(filter *Process, opts ...TransportProtocol) map[string]struct{} {
	rm := make(map[string]struct{})
	ps, _ := List(filter, opts...)
	for _, p := range ps {
		rm[p.LocalPort] = struct{}{}
		rm[p.RemotePort] = struct{}{}
	}
	return rm
}

type Ports struct {
	mu        sync.Mutex // guards the following
	beingUsed map[string]struct{}
}

func NewPorts() *Ports {
	p := &Ports{}
	p.beingUsed = make(map[string]struct{})
	return p
}

func (p *Ports) Exist(port string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, ok := p.beingUsed[port]
	return ok
}

func (p *Ports) Add(ports ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, port := range ports {
		p.beingUsed[port] = struct{}{}
	}
}

func (p *Ports) Free(ports ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, port := range ports {
		delete(p.beingUsed, port)
	}
}

// Refresh refreshes ports that are being used by the OS.
// You would use as follows:
//
//	var (
//		globalPorts     = NewPorts()
//		refreshInterval = 10 * time.Second
//	)
//	func init() {
//		globalPorts.Refresh()
//		go func() {
//			for {
//				select {
//				case <-time.After(refreshInterval):
//					globalPorts.Refresh()
//				}
//			}
//		}()
//	}
//
func (p *Ports) Refresh() {
	p.mu.Lock()
	p.beingUsed = ListPorts(nil, TCP, TCP6)
	p.mu.Unlock()
}

func getFreePort(opts ...TransportProtocol) (string, error) {
	fp := ""
	var errMsg error
	for _, opt := range opts {
		protocolStr := ""
		switch opt {
		case TCP:
			protocolStr = "tcp"
		case TCP6:
			protocolStr = "tcp6"
		}
		addr, err := net.ResolveTCPAddr(protocolStr, "localhost:0")
		if err != nil {
			errMsg = err
			continue
		}
		l, err := net.ListenTCP(protocolStr, addr)
		if err != nil {
			errMsg = err
			continue
		}
		pd := l.Addr().(*net.TCPAddr).Port
		l.Close()
		fp = ":" + strconv.Itoa(pd)
		break
	}
	return fp, errMsg
}

// GetFreePorts returns multiple free ports from OS.
func GetFreePorts(num int, opts ...TransportProtocol) ([]string, error) {
	try := 0
	rm := make(map[string]struct{})
	for len(rm) != num {
		if try > 150 {
			return nil, fmt.Errorf("too many tries to find free ports")
		}
		try++
		p, err := getFreePort(opts...)
		if err != nil {
			return nil, err
		}
		rm[p] = struct{}{}
	}
	rs := []string{}
	for p := range rm {
		rs = append(rs, p)
	}
	sort.Strings(rs)
	return rs, nil
}
