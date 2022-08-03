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
	if pid < 0 {
		pid = -pid
	}
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

	em := map[string]string{}

	for _, p := range ft {
		for _, conn := range p.Connections {
			if conn.Self.Pid == 0 || conn.Peer.Pid == 0 || // ignore kernel process
				conn.Self.Pid == 1 || conn.Peer.Pid == 1 || // ignore launchd processes
				conn.Self.Pid == conn.Peer.Pid || // ignore inter-process connections
				query.pid == 0 && conn.Peer.Pid >= math.MaxInt32 { // ignore data connections for the "all process" query
				continue
			}

			include[conn.Self.Pid] = struct{}{}

			if conn.Peer.Pid < 0 { // peer is remote host or listener
				host, port, _ := net.SplitHostPort(conn.Peer.Name)

				dir := "forward"
				// name for listen port is device inode: on linux decimal and on darwin hexadecimal
				if _, err := strconv.Atoi(conn.Self.Name); err == nil || conn.Self.Name[0:2] == "0x" { // listen socket
					dir = "back"
				}

				hosts += fmt.Sprintf(`
    %d [shape=cds height=0.5 label="%s:%s\n%s"]`,
					conn.Peer.Pid,
					conn.Type,
					port,
					hostname(host),
				)
				if hostNode == 0 {
					hostNode = conn.Peer.Pid
				}

				// TODO: host arrow on east/right edge
				hostEdges += fmt.Sprintf(`
  %d -> %d [dir=%s color=%q tooltip="%s ‑> %s\n%s"]`, // non-breaking space/hyphen
					conn.Peer.Pid,
					conn.Self.Pid,
					dir,
					color(conn.Self.Pid),
					conn.Type+":"+conn.Peer.Name,
					conn.Self.Name,
					longname(pt, conn.Self.Pid),
				)
			} else if conn.Peer.Pid >= math.MaxInt32 { // peer is data
				peer := conn.Type + ":" + conn.Peer.Name

				datas += fmt.Sprintf(`
    %d [shape=note color=%q label=%q]`,
					conn.Peer.Pid,
					color(conn.Peer.Pid),
					peer,
				)
				if dataNode == 0 {
					dataNode = conn.Peer.Pid
				}

				// show edge for data connections only once
				id := fmt.Sprintf("%d -> %d", conn.Self.Pid, conn.Peer.Pid)
				if _, ok := em[id]; !ok {
					em[id] = ""
					dataEdges += fmt.Sprintf(`
  %d -> %d [dir=forward color=%q tooltip="%s\n%s"]`,
						conn.Self.Pid,
						conn.Peer.Pid,
						color(conn.Peer.Pid),
						longname(pt, conn.Self.Pid),
						peer,
					)
				}
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

				if conn.Type == "parent" {
					processEdges[depth] += fmt.Sprintf(`
  %d -> %d [dir=forward tooltip="%s ‑> %s\n"]`, // non-breaking space/hyphen
						conn.Self.Pid,
						conn.Peer.Pid,
						shortname(pt, conn.Self.Pid),
						shortname(pt, conn.Peer.Pid),
					)
					continue
				}

				// show edge for inter-process connections only once
				id := fmt.Sprintf("%d -> %d", conn.Self.Pid, conn.Peer.Pid)
				di := fmt.Sprintf("%d -> %d", conn.Peer.Pid, conn.Self.Pid)

				_, ok := em[id]
				if ok {
					em[id] += fmt.Sprintf("%s:%s ‑> %s\n", // non-breaking space/hyphen
						conn.Type,
						conn.Self.Name,
						conn.Peer.Name,
					)
				} else if _, ok = em[di]; ok {
					em[di] += fmt.Sprintf("%s:%s ‑> %s\n", // non-breaking space/hyphen
						conn.Type,
						conn.Peer.Name,
						conn.Self.Name,
					)
				} else {
					em[id] = fmt.Sprintf("%s ‑> %s\n%s:%s ‑> %s\n", // non-breaking space/hyphen
						shortname(pt, conn.Self.Pid),
						shortname(pt, conn.Peer.Pid),
						conn.Type,
						conn.Self.Name,
						conn.Peer.Name,
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
      %[2]d [shape=rect style=rounded color=%q URL="http://localhost:%d/gomon?pid=%[2]d" label="%[1]s\n%d" tooltip=%[5]q]`,
			pt[pid].Id.Name,
			pid,
			color(pid),
			core.Flags.Port,
			longname(pt, pid),
		)
		processes[len(p.Ancestors)] += node

		depth := len(pt[pid].Ancestors)

		for edge, tooltip := range em {
			if strings.Fields(edge)[0] == strconv.Itoa(int(pid)) {
				if tooltip != "" {
					processEdges[depth] += fmt.Sprintf(`
  %s [dir=both color=%q tooltip=%q]`, // TODO: arrow on left/west edge
						edge,
						color(pid),
						tooltip,
					)
				}
				delete(em, edge)
			}
		}
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
  edge [penwidth=1.5 arrowsize=0.5]
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

	pids := process.FlatTree(process.FindTree(process.BuildTree(pt), pid)) // descendants
	for _, pid := range pids {
		ft[pid] = pt[pid]
	}
	return ft
}

// longname formats the full Executable name and pid.
func longname(pt process.Table, pid Pid) string {
	if p, ok := pt[pid]; ok {
		name := p.Executable
		if name == "" {
			name = p.Id.Name
		}
		return fmt.Sprintf("%s[%d]", name, pid)
	}
	return ""
}

// shortname formats process name and pid.
func shortname(pt process.Table, pid Pid) string {
	if p, ok := pt[pid]; ok {
		return fmt.Sprintf("%s[%d]", p.Id.Name, pid)
	}
	return ""
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
