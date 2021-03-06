// Copyright © 2021 The Gomon Project.

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/zosmac/gomon/core"
	"github.com/zosmac/gomon/process"
)

var (
	// hnMap caches resolver host name lookup.
	hnMap  = map[string]string{}
	hnLock sync.Mutex

	// graphviz colors for nodes and edges
	colors = []string{"#7777DD", "#FF6666", "#00AA00", "#6688FF", "#00BBBB", "#BB44BB", "#AAAA00", "#448888", "#886688", "#888844"}
)

// color defines the color for graphviz nodes and edges
func color(pid Pid) string {
	color := "#000000"
	if pid > 0 {
		color = colors[(int(pid-1))%len(colors)]
	}
	return color
}

type (
	// Pid alias for Pid in process package.
	Pid = process.Pid

	// query from http request.
	query struct {
		pid Pid
	}
)

// NodeGraph produces the process connections node graph.
func NodeGraph(req *http.Request) []byte {
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			buf = buf[:n]
			core.LogError(fmt.Errorf("NodeGraph() panicked, %v\n%s", r, buf))
		}
	}()

	var (
		clusterEdges string
		hosts        string
		hostNode     Pid
		hostEdges    string
		processes    []string
		processNodes []Pid
		processEdges []string
		datas        string
		dataNode     Pid
		dataEdges    string
		include      = map[Pid]struct{}{} // record which processes have a connection to include in report
	)

	query, _ := parseQuery(req)

	ft := process.Table{}
	pt := process.BuildTable()
	process.Connections(pt)

	if query.pid > 0 && pt[query.pid] == nil {
		query.pid = 0 // reset to default
	}
	if query.pid > 0 { // build this process' "extended family"
		ft = family(pt, query.pid)
	} else { // only consider non-daemon and remote host connected processes
		for pid, p := range pt {
			if p.Ppid > 1 {
				for pid, p := range family(pt, pid) {
					ft[pid] = p
				}
			}
			for _, conn := range p.Connections {
				if conn.Peer.Pid < 0 {
					ft[conn.Self.Pid] = pt[conn.Self.Pid]
				}
			}
		}
	}

	em := map[string]struct{}{}

	for _, p := range ft {
		for _, conn := range p.Connections {
			if conn.Self.Pid == 0 || conn.Peer.Pid == 0 || // ignore kernel process
				conn.Self.Pid == 1 || conn.Peer.Pid == 1 || // ignore launchd processes
				conn.Self.Pid == conn.Peer.Pid || // ignore inter-process connections
				query.pid == 0 && conn.Peer.Pid >= math.MaxInt32 { // ignore data connections for the "all process" query
				continue
			}

			var dir string // graphviz arrow direction
			switch conn.Direction {
			case "-->>":
				dir = "forward"
			case "<<--":
				dir = "back"
			case "<-->":
				dir = "both"
			default:
				dir = "none"
			}

			include[conn.Self.Pid] = struct{}{}

			if conn.Peer.Pid < 0 { // peer is remote host or listener
				host, port, _ := net.SplitHostPort(conn.Peer.Name)
				peer := conn.Type + ":" + conn.Peer.Name

				color := "black"
				dir := "both"
				if conn.Self.Name[0:2] == "0x" { // listen socket
					color = "red"
					dir = "forward"
				}

				hosts += fmt.Sprintf(`
    %d [label="%s:%s\n%s" color=%s shape=cds height=0.5]`,
					conn.Peer.Pid,
					conn.Type,
					port,
					hostname(host),
					color,
				)
				if hostNode == 0 {
					hostNode = conn.Peer.Pid
				}

				hostEdges += fmt.Sprintf(`
  %d -> %d [tooltip="%s\n%s" dir=%s]`,
					conn.Peer.Pid,
					conn.Self.Pid,
					peer+"->"+conn.Self.Name,
					longname(pt, conn.Self.Pid),
					dir,
				)
			} else if conn.Peer.Pid >= math.MaxInt32 { // peer is data
				peer := conn.Type + ":" + conn.Peer.Name

				var color string
				switch conn.Type {
				case "DIR":
					color = "#00FF00"
				case "REG":
					color = "#BBBB99"
				default:
					color = "#99BBBB"
				}

				datas += fmt.Sprintf(`
    %d [label=%q color=%q shape=note]`,
					conn.Peer.Pid,
					peer,
					color,
				)
				if dataNode == 0 {
					dataNode = conn.Peer.Pid
				}

				dataEdges += fmt.Sprintf(`
  %d -> %d [tooltip="%s\n%s" dir=%s color=%q]`,
					conn.Self.Pid,
					conn.Peer.Pid,
					longname(pt, conn.Self.Pid),
					peer,
					dir,
					color,
				)
			} else { // peer is process
				include[conn.Peer.Pid] = struct{}{}

				depth := len(pt[conn.Self.Pid].Ancestors)
				for i := len(processNodes); i <= depth; i++ {
					processNodes = append(processNodes, 0)
					processEdges = append(processEdges, "")
				}
				if processNodes[depth] == 0 {
					processNodes[depth] = conn.Self.Pid
				}

				color := color(conn.Self.Pid)
				if conn.Type == "parent" {
					color = "black"
				}

				// show bidirectional connection only once
				id := fmt.Sprintf("%d->%d", conn.Self.Pid, conn.Peer.Pid)
				di := fmt.Sprintf("%d->%d", conn.Peer.Pid, conn.Self.Pid)

				_, ok := em[id]
				if !ok {
					_, ok = em[di]
				}
				if !ok {
					processEdges[depth] += fmt.Sprintf(`
  %d -> %d [tooltip="%s\n%s\n%s" dir=%s color=%q]`,
						conn.Self.Pid,
						conn.Peer.Pid,
						conn.Type+":"+conn.Self.Name+"->"+conn.Peer.Name,
						shortname(pt, conn.Self.Pid),
						shortname(pt, conn.Peer.Pid),
						dir,
						color,
					)
				}
			}
		}
	}

	for pid, p := range pt {
		if _, ok := include[pid]; !ok {
			continue
		}

		for i := len(processes); i <= len(p.Ancestors); i++ {
			processes = append(processes, fmt.Sprintf(`
    subgraph cluster_processes_%d {
      label="Process depth %[1]d" rank=same fontsize=11 penwidth=3.0 pencolor="#5599BB"`,
				i+1))
		}

		node := fmt.Sprintf(`
      %d [label="%s\n%[1]d" tooltip=%[3]q color=%q URL="http://localhost:%d/gomon?pid=%[1]d" shape=rect style=rounded]`,
			pid,
			filepath.Base(pt[pid].Executable),
			longname(pt, pid),
			color(pid),
			core.Flags.Port,
		)
		processes[len(p.Ancestors)] += node
	}

	for i := range processes {
		processes[i] += "\n    }"
	}

	if len(processNodes) > 0 {
		if hostNode != 0 {
			clusterEdges += fmt.Sprintf(`
  %d -> %d [style=invis ltail="cluster_hosts" lhead="cluster_processes_1"]`,
				hostNode,
				processNodes[0],
			)
		}
		for i := range processNodes[:len(processNodes)-1] {
			clusterEdges += fmt.Sprintf(`
  %d -> %d [style=invis ltail="cluster_processes_%d" lhead="cluster_processes_%d"]`,
				processNodes[i],
				processNodes[i+1],
				i+1,
				i+2,
			)
		}
		if dataNode != 0 {
			clusterEdges += fmt.Sprintf(`
  %d -> %d [style=invis ltail="cluster_processes_%d" lhead="cluster_files"]`,
				processNodes[len(processNodes)-1],
				dataNode,
				len(processNodes),
			)
		}
	}

	var label string
	if query.pid > 0 {
		label = fmt.Sprintf("Inter-Process Connections for Process %s on ", shortname(pt, query.pid))
	} else {
		label = "Remote Hosts and Inter-Process Connections for "
	}
	label += core.Hostname + time.Now().Local().Format(", Mon Jan 02 2006 at 03:04:05PM MST")

	return dot(`digraph "` + label + `" {
  id="\G"
  label="\G"
  labelloc=t
  labeljust=l
  rankdir=LR
  newrank=true
  compound=true
  constraint=false
  remincross=false
  ordering=out
  nodesep=0.05
  ranksep="2.0"
  node [fontname=helvetica fontsize=7 height=0.2 width=1.5]
  edge [arrowsize=0.5]
  subgraph cluster_hosts {
    label="External Connections" rank=same fontsize=11 penwidth=3.0 pencolor="#BB5599"` +
		hosts + `
  }
  subgraph cluster_processes {
    label=Processes fontsize=11 penwidth=3.0 pencolor="#5599BB"` +
		strings.Join(processes, "") + `
  }
  subgraph cluster_files {
    label="Open Files" rank=max fontsize=11 penwidth=3.0 pencolor="#99BB55"` +
		datas + `
  }` +
		clusterEdges +
		hostEdges +
		strings.Join(processEdges, "") +
		dataEdges + `
}`)
}

// dot calls the Graphviz dot command to render the process NodeGraph as SVG.
func dot(graphviz string) []byte {
	cmd := exec.Command("dot", "-v", "-Tsvgz")
	cmd.Stdin = bytes.NewBufferString(graphviz)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		core.LogError(fmt.Errorf("dot command failed %w\n%s", err, stderr.Bytes()))
		sc := bufio.NewScanner(strings.NewReader(graphviz))
		for i := 1; sc.Scan(); i++ {
			fmt.Fprintf(os.Stderr, "%4.d %s\n", i, sc.Text())
		}
		return nil
	}

	return stdout.Bytes()
}

// parseQuery extracts the query from the HTTP request.
func parseQuery(r *http.Request) (query, error) {
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return query{}, err
	}
	var pid int
	if v, ok := values["pid"]; ok && len(v) > 0 {
		pid, _ = strconv.Atoi(v[0])
	}
	return query{
		pid: Pid(pid),
	}, nil
}

// family identifies all of the processes related to a process.
func family(pt process.Table, pid Pid) process.Table {
	ft := process.Table{pid: pt[pid]}
	for pid := pt[pid].Ppid; pid > 1; pid = pt[pid].Ppid { // ancestors
		ft[pid] = pt[pid]
	}
	pids := process.FlatTree(process.FindTree(process.BuildTree(pt), pid), 0) // descendants
	for _, pid := range pids {
		ft[pid] = pt[pid]
	}
	return ft
}

// longname formats the full Executable name and pid.
func longname(pt process.Table, pid Pid) string {
	return fmt.Sprintf("%s[%d]", pt[pid].Executable, pid)
}

// shortname formats the base Executable name and pid.
func shortname(pt process.Table, pid Pid) string {
	return fmt.Sprintf("%s[%d]", filepath.Base(pt[pid].Executable), pid)
}

// hostname resolves the host name for an ip address.
func hostname(ip string) string {
	hnLock.Lock()
	defer hnLock.Unlock()

	if host, ok := hnMap[ip]; ok {
		return host
	}

	hnMap[ip] = ip
	go func() { // initiate hostname lookup
		if hosts, err := net.LookupAddr(ip); err == nil {
			hnLock.Lock()
			hnMap[ip] = hosts[0]
			hnLock.Unlock()
		}
	}()

	return ip
}
