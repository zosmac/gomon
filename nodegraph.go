// Copyright © 2021-2023 The Gomon Project.

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
	"time"

	"github.com/zosmac/gocore"
	"github.com/zosmac/gomon/process"
)

type (
	// Pid alias for Pid in process package.
	Pid = process.Pid

	// query from http request.
	query struct {
		pid Pid
	}
)

var (
	// colors on HSV spectrum that work well in light and dark mode
	colors = []string{
		"0.0 0.75 0.80",
		"0.1 0.75 0.75",
		"0.2 0.75 0.7",
		"0.3 0.75 0.75",
		"0.4 0.75 0.75",
		"0.5 0.75 0.75",
		"0.6 0.75 0.9",
		"0.7 0.75 1.0", // blue needs to be a bit brighter
		"0.8 0.75 0.9",
		"0.9 0.75 0.85",
	}
)

// color defines the color for graphviz nodes and edges
func color(pid Pid) string {
	var color string
	if pid < 0 {
		pid = -pid
	}
	if pid > 0 {
		color = colors[(int(pid-1))%len(colors)]
	}
	return color
}

// NodeGraph produces the process connections node graph.
func NodeGraph(req *http.Request) []byte {
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			buf = buf[:n]
			gocore.Error("nodegraph", fmt.Errorf("%v", r), map[string]string{
				"stacktrace": string(buf),
			}).Err()
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
		nodes        = map[Pid]struct{}{}
	)

	tb := process.BuildTable()
	tr := process.BuildTree(tb)
	process.Connections(tb)

	query, _ := parseQuery(req)
	if query.pid != 0 && tb[query.pid] == nil {
		query.pid = 0 // reset to default
	}

	pt := process.Table{}
	if query.pid > 0 { // build this process' "extended family"
		pt = family(tb, tr, query.pid)
	} else { // only consider non-daemon and remote host connected processes
		for pid, p := range tb {
			if p.Ppid > 1 {
				for pid, p := range family(tb, tr, pid) {
					pt[pid] = p
				}
			}
			for _, conn := range p.Connections {
				if conn.Peer.Pid < 0 {
					pt[conn.Self.Pid] = tb[conn.Self.Pid]
				}
			}
		}
	}

	em := map[string]string{}

	for _, p := range pt {
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

				if _, ok := nodes[conn.Peer.Pid]; !ok {
					nodes[conn.Peer.Pid] = struct{}{}
					hosts += fmt.Sprintf(`
    %d [shape=cds style=filled fillcolor=%q height=0.6 width=2 label="%s:%s\n%s" tooltip=%q]`,
						conn.Peer.Pid,
						color(conn.Peer.Pid),
						conn.Type,
						port,
						gocore.Hostname(host),
						conn.Peer.Name,
					)
				}
				if hostNode == 0 {
					hostNode = conn.Peer.Pid
				}

				// TODO: host arrow on east/right edge
				hostEdges += fmt.Sprintf(`
  %d -> %d [dir=%s color=%q tooltip="%s&#10142;%s\n%s"]`, // non-breaking space/hyphen
					conn.Peer.Pid,
					conn.Self.Pid,
					dir,
					color(conn.Self.Pid)+";0.5:"+color(conn.Peer.Pid),
					conn.Type+":"+conn.Peer.Name,
					conn.Self.Name,
					shortname(tb, conn.Self.Pid),
				)
			} else if conn.Peer.Pid >= math.MaxInt32 { // peer is data
				peer := conn.Type + ":" + conn.Peer.Name

				datas += fmt.Sprintf(`
    %d [shape=note style=filled fillcolor=%q height=0.2 label=%q tooltip=%q]`,
					conn.Peer.Pid,
					color(conn.Peer.Pid),
					peer,
					conn.Peer.Name,
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
						color(conn.Peer.Pid)+";0.5:"+color(conn.Self.Pid),
						shortname(tb, conn.Self.Pid),
						peer,
					)
				}
			} else { // peer is process
				include[conn.Peer.Pid] = struct{}{}

				depth := len(tb[conn.Self.Pid].Ancestors)
				for i := len(processNodes); i <= depth; i++ {
					processNodes = append(processNodes, 0)
					processEdges = append(processEdges, "")
				}
				if processNodes[depth] == 0 {
					processNodes[depth] = conn.Self.Pid
				}

				if conn.Type == "parent" {
					processEdges[depth] += fmt.Sprintf(`
  %d -> %d [class=parent dir=forward tooltip="%s&#10142;%s\n"]`, // non-breaking space/hyphen
						conn.Self.Pid,
						conn.Peer.Pid,
						shortname(tb, conn.Self.Pid),
						shortname(tb, conn.Peer.Pid),
					)
					continue
				}

				// show edge for inter-process connections only once
				id := fmt.Sprintf("%d -> %d", conn.Self.Pid, conn.Peer.Pid)
				di := fmt.Sprintf("%d -> %d", conn.Peer.Pid, conn.Self.Pid)

				_, ok := em[id]
				if ok {
					em[id] += fmt.Sprintf("%s:%s&#10142;%s\n", // non-breaking space/hyphen
						conn.Type,
						conn.Self.Name,
						conn.Peer.Name,
					)
				} else if _, ok = em[di]; ok {
					em[di] += fmt.Sprintf("%s:%s&#10142;%s\n", // non-breaking space/hyphen
						conn.Type,
						conn.Peer.Name,
						conn.Self.Name,
					)
				} else {
					em[id] = fmt.Sprintf("%s&#10142;%s\n%s:%s&#10142;%s\n", // non-breaking space/hyphen
						shortname(tb, conn.Self.Pid),
						shortname(tb, conn.Peer.Pid),
						conn.Type,
						conn.Self.Name,
						conn.Peer.Name,
					)
				}
			}
		}
	}

	for pid, p := range tb {
		if _, ok := include[pid]; !ok {
			continue
		}

		for i := len(processes); i <= len(p.Ancestors); i++ {
			processes = append(processes, fmt.Sprintf(`
    subgraph processes_%d {
      cluster=true rank=same label="Process depth %[1]d"`,
				i+1,
			))
		}

		node := fmt.Sprintf(`
      %d [shape=rect style="rounded,filled" fillcolor=%q height=0.3 width=1 URL="%s://localhost:%d/gomon?pid=\N" label="%s\n\N" tooltip=%q]`,
			pid,
			color(pid),
			scheme,
			flags.port,
			tb[pid].Id.Name,
			longname(tb, pid),
		)
		processes[len(p.Ancestors)] += node

		depth := len(tb[pid].Ancestors)

		for edge, tooltip := range em {
			fields := strings.Fields(edge)
			self, _ := strconv.Atoi(fields[0])
			peer, _ := strconv.Atoi(fields[2])
			if Pid(self) == pid {
				if tooltip != "" {
					processEdges[depth] += fmt.Sprintf(`
  %s [dir=both color=%q tooltip=%q]`,
						edge,
						color(Pid(peer))+";0.5:"+color(Pid(self)),
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
  %d -> %d [style=invis ltail="hosts" lhead="processes_1"]`,
				hostNode,
				processNodes[0],
			)
		}
		for i := range processNodes[:len(processNodes)-1] {
			clusterEdges += fmt.Sprintf(`
  %d -> %d [style=invis ltail="processes_%d" lhead="processes_%d"]`,
				processNodes[i],
				processNodes[i+1],
				i+1,
				i+2,
			)
		}
		if dataNode != 0 {
			clusterEdges += fmt.Sprintf(`
  %d -> %d [style=invis ltail="processes_%d" lhead="files"]`,
				processNodes[len(processNodes)-1],
				dataNode,
				len(processNodes),
			)
		}
	}

	var pslabel string
	if query.pid > 0 {
		pslabel = " Process: " + shortname(tb, query.pid)
	}

	glabel := fmt.Sprintf(
		`"External and Inter-Process Connections\lHost: %s%s%s`,
		gocore.Host,
		pslabel,
		time.Now().Local().Format(`\lMon Jan 02 2006 at 03:04:05PM MST\l"`),
	)

	return dot(`digraph "Gomon Process Connections Nodegraph" {
  stylesheet="/assets/mode.css"
  label=` + glabel + `
  labelloc=t
  labeljust=l
  rankdir=LR
  newrank=true
  compound=true
  constraint=false
  ordering=out
  nodesep=0.03
  ranksep=2
  node [margin=0]
  subgraph hosts {
    cluster=true rank=source label="External Connections"` +
		hosts + `
  }
  subgraph processes {
	cluster=true label=Processes` +
		strings.Join(processes, "") + `
  }
  subgraph files {
	cluster=true rank=sink label="Open Files"` +
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
		gocore.Error("dot", err, map[string]string{
			"stderr": stderr.String(),
		}).Err()
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

// family identifies all of the ancestor and children processes of a process.
func family(tb process.Table, tr process.Tree, pid Pid) process.Table {
	pt := process.Table{}
	for pid := pid; pid > 0; pid = tb[pid].Ppid { // ancestors
		pt[pid] = tb[pid]
	}

	tr = tr.FindTree(pid)
	o := func(node Pid, pt process.Table) int {
		return order(node, tr, pt)
	}

	pids := tr.Flatten(tb, o)
	for _, pid := range pids {
		pt[pid] = tb[pid]
	}

	return pt
}

// order returns the process' depth in the tree for sorting.
func order(node Pid, tr process.Tree, _ process.Table) int {
	var depth int
	for _, tr := range tr {
		dt := depthTree(tr) + 1
		if depth < dt {
			depth = dt
		}
	}
	return depth
}

// depthTree enables sort of deepest process trees first.
func depthTree(tr process.Tree) int {
	depth := 0
	for _, tr := range tr {
		dt := depthTree(tr) + 1
		if depth < dt {
			depth = dt
		}
	}
	return depth
}

// longname formats the full Executable name and pid.
func longname(tb process.Table, pid Pid) string {
	if p, ok := tb[pid]; ok {
		name := p.Executable
		if name == "" {
			name = p.Id.Name
		}
		return fmt.Sprintf("%s[%d]", name, pid)
	}
	return ""
}

// shortname formats process name and pid.
func shortname(tb process.Table, pid Pid) string {
	if p, ok := tb[pid]; ok {
		return fmt.Sprintf("%s[%d]", p.Id.Name, pid)
	}
	return ""
}
