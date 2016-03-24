package ss

import (
	"bufio"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
)

type colIdx int

const (
	// http://www.onlamp.com/pub/a/linux/2000/11/16/LinuxAdmin.html
	// [`sl` `local_address` `rem_address` `st` `tx_queue` `rx_queue` `tr`
	// `tm->when` `retrnsmt` `uid` `timeout` `inode`]
	idx_sl colIdx = iota
	idx_local_address
	idx_remote_address
	idx_st
	idx_tx_queue_rx_queue
	idx_tr_tm_when
	idx_retrnsmt
	idx_uid
	idx_timeout
	idx_inode
)

var (
	protocolToPath = map[TransportProtocol]string{
		TCP:  "/proc/net/tcp",
		TCP6: "/proc/net/tcp6",
	}

	// RPC_SHOW_SOCK
	// https://github.com/torvalds/linux/blob/master/include/trace/events/sunrpc.h
	stToState = map[string]string{
		"01": "ESTABLISHED",
		"02": "SYN_SENT",
		"03": "SYN_RECV",
		"04": "FIN_WAIT1",
		"05": "FIN_WAIT2",
		"06": "TIME_WAIT",
		"07": "CLOSE",
		"08": "CLOSE_WAIT",
		"09": "LAST_ACK",
		"0A": "LISTEN",
		"0B": "CLOSING",
	}
)

// readProcNet reads '/proc/net/*'.
func readProcNet(opt TransportProtocol) ([][]string, error) {
	f, err := os.OpenFile(protocolToPath[opt], os.O_RDONLY, 0444)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fields := [][]string{}

	scanner := bufio.NewScanner(f)

	colN := 0
	for scanner.Scan() {
		txt := scanner.Text()
		if len(txt) == 0 {
			continue
		}

		fs := strings.Fields(txt)
		if colN == 0 {
			if fs[0] != "sl" {
				return nil, fmt.Errorf("first line must be columns but got = %#q", fs)
			}
			colN = len(fs)
		}

		if colN > len(fs) {
			continue
		}

		fields = append(fields, fs)
	}

	if err := scanner.Err(); err != nil {
		return fields, err
	}

	return fields, nil
}

// readProcFd reads '/proc/*/fd/*' to grab process IDs.
func readProcFd() ([]string, error) {
	// returns the names of all files matching pattern
	// or nil if there is no matching file
	fs, err := filepath.Glob("/proc/[0-9]*/fd/[0-9]*")
	if err != nil {
		return nil, err
	}
	return fs, nil
}

// list parses raw data from readProcNet, readProcFd functions.
func list(opt TransportProtocol) ([]Process, error) {
	fields, err := readProcNet(opt)
	if err != nil {
		return nil, err
	}
	procNets := fields[1:]

	procFds, err := readProcFd()
	if err != nil {
		return nil, err
	}

	pChan, errChan := make(chan Process), make(chan error)
	for _, sl := range procNets {

		go func(sl []string) {

			p := Process{}
			p.Protocol = protocolToString[opt]

			inode := sl[idx_inode]
			for _, procPath := range procFds {
				sym, _ := os.Readlink(procPath)
				if !strings.Contains(sym, inode) {
					continue
				}
				n, err := strconv.Atoi(strings.Split(procPath, "/")[2])
				if err != nil {
					errChan <- err
					return
				}
				p.PID = n
				break
			}

			pName, _ := os.Readlink(fmt.Sprintf("/proc/%d/exe", p.PID))
			p.Program = pName

			var parseFunc func(string) (string, string, error)
			switch opt {
			case TCP:
				parseFunc = parseLittleEndianIpv4
			case TCP6:
				parseFunc = parseLittleEndianIpv6
			}

			lIP, lPort, err := parseFunc(sl[idx_local_address])
			if err != nil {
				errChan <- err
				return
			}
			rIP, rPort, err := parseFunc(sl[idx_remote_address])
			if err != nil {
				errChan <- err
				return
			}

			p.LocalIP = lIP
			p.LocalPort = lPort
			p.RemoteIP = rIP
			p.RemotePort = rPort

			p.State = stToState[sl[idx_st]]

			u, err := user.LookupId(sl[idx_uid])
			if err != nil {
				errChan <- err
				return
			}

			p.User = *u

			pChan <- p

		}(sl)

	}

	ps := []Process{}
	cn, limit := 0, len(procNets)
	for cn != limit {
		select {
		case err := <-errChan:
			return nil, err
		case p := <-pChan:
			ps = append(ps, p)
			cn++
		}
	}

	close(pChan)
	close(errChan)

	return ps, nil
}
