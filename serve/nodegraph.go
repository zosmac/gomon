// Copyright © 2021-2023 The Gomon Project.

package serve

/*
//#cgo CFLAGS: -I/usr/local/include
//#cgo LDFLAGS: -L/usr/local/lib -lgvc -lcgraph

//#include <graphviz/gvc.h>
//#include <stdlib.h>
*/
//import "C"

import (
	"bufio"
	"bytes"
	"cmp"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
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
		"0.0 0.75 0.8  0.5",
		"0.1 0.75 0.75 0.5",
		"0.2 0.75 0.7  0.5",
		"0.3 0.75 0.75 0.5",
		"0.4 0.75 0.75 0.5",
		"0.5 0.75 0.75 0.5",
		"0.6 0.75 0.9  0.5",
		"0.7 0.75 1.0  0.5", // blue needs to be a bit brighter
		"0.8 0.75 0.9  0.5",
		"0.9 0.75 0.85 0.5",
	}

	// prevCPU is used to limit reporting only of processes that consumed CPU since the previous measurement.
	prevCPU = map[Pid]time.Duration{}
)

// color defines the color for graphviz nodes and edges.
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

// Nodegraph produces the process connections node graph.
func Nodegraph(req *http.Request) []byte {
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
		clusterNodes [][2]string                // One node in each cluster is linked to a node in the
		clusterEdges string                     //  next to maintain left to right order of clusters.
		hosts        = map[Pid]string{}         // The host (IP) nodes in the leftmost cluster.
		prcss        = map[int]map[Pid]string{} // The process nodes in the process clusters.
		datas        = map[Pid]string{}         // The file, unix socket, pipe and kernel connections in the rightmost cluster.
		include      = process.Table{}          // Processes that have a connection to include in report.
		edges        = map[[2]Pid]string{}
		edgeTooltips = map[[2]Pid]map[string]struct{}{}
	)

	tb := process.BuildTable()
	tr := process.BuildTree(tb)
	process.Connections(tb)

	currCPU := map[Pid]time.Duration{}
	for pid, p := range tb {
		currCPU[pid] = p.Total
	}

	query, _ := parseQuery(req)
	if query.pid != 0 && tb[query.pid] == nil {
		query.pid = 0 // reset to default
	}

	gocore.Error("nodegraph", nil, map[string]string{
		"pid": query.pid.String(),
	}).Info()

	pt := process.Table{}
	if query.pid > 0 { // build this process' "extended family"
		for _, pid := range tr.Family(query.pid).All() {
			pt[pid] = tb[pid]
		}
		for _, p := range tb {
			for _, conn := range p.Connections {
				if conn.Peer.Pid == query.pid {
					for _, pid := range tr.Ancestors(conn.Self.Pid) {
						pt[pid] = tb[pid]
					}
					pt[conn.Self.Pid] = tb[conn.Self.Pid]
				}
			}
		}
	} else { // only report non-daemon, remote host connected, and cpu consuming processes
		for pid, p := range tb {
			if pcpu, ok := prevCPU[pid]; ok && pcpu < currCPU[pid] {
				pt[pid] = tb[pid]
			}
			if p.Ppid > 1 {
				for _, pid := range tr.Family(pid).All() {
					pt[pid] = tb[pid]
				}
			}
			for _, conn := range p.Connections {
				if conn.Peer.Pid < 0 {
					pt[conn.Self.Pid] = tb[conn.Self.Pid]
				}
			}
		}
	}

	prevCPU = currCPU

	for pid, p := range pt {
		include[pid] = p
		for _, conn := range p.Connections {
			if conn.Self.Pid == 0 || conn.Peer.Pid == 0 || // ignore kernel process
				conn.Self.Pid == 1 || // ignore launchd processes
				conn.Self.Pid == conn.Peer.Pid || // ignore inter-process connections
				query.pid == 0 && conn.Peer.Pid >= math.MaxInt32 || // ignore data connections for the "all process" query
				(query.pid > 0 && query.pid != conn.Self.Pid && // ignore hosts and datas of connected processes
					(conn.Peer.Pid < 0 || conn.Peer.Pid >= math.MaxInt32)) {
				continue
			}

			if conn.Peer.Pid < 0 { // peer is remote host or listener
				host, port, _ := net.SplitHostPort(conn.Peer.Name)

				if _, ok := hosts[conn.Peer.Pid]; !ok {
					hosts[conn.Peer.Pid] = fmt.Sprintf(
						`[shape=cds style=filled fillcolor=%q height=0.6 width=2 label="%s:%s\n%s" tooltip=%q]`,
						color(conn.Peer.Pid),
						conn.Type,
						port,
						gocore.Hostname(host),
						conn.Peer.Name,
					)
				}

				// flip the source and target to get Host shown to left in node graph
				id := [2]Pid{conn.Peer.Pid, conn.Self.Pid}
				if _, ok := edgeTooltips[id]; !ok {
					edgeTooltips[id] = map[string]struct{}{}
				}
				edgeTooltips[id][fmt.Sprintf(
					"%s:%s&#10142;%s",
					conn.Type,
					conn.Peer.Name,
					conn.Self.Name,
				)] = struct{}{}
			} else if conn.Peer.Pid >= math.MaxInt32 { // peer is data
				peer := conn.Type + ":" + conn.Peer.Name

				if _, ok := datas[conn.Peer.Pid]; !ok {
					datas[conn.Peer.Pid] = fmt.Sprintf(
						`[shape=note style=filled fillcolor=%q height=0.2 label=%q tooltip=%q]`,
						color(conn.Peer.Pid),
						peer,
						conn.Peer.Name,
					)
				}

				id := [2]Pid{conn.Self.Pid, conn.Peer.Pid}
				if _, ok := edgeTooltips[id]; !ok {
					edgeTooltips[id] = map[string]struct{}{}
				}
				edgeTooltips[id][fmt.Sprintf(
					"%s&#10142;%s",
					shortname(tb, conn.Self.Pid),
					peer,
				)] = struct{}{}
			} else { // peer is process
				include[conn.Peer.Pid] = tb[conn.Peer.Pid]

				// show edge for inter-process connections only once
				self, peer := conn.Self.Name, conn.Peer.Name
				selfPid, peerPid := conn.Self.Pid, conn.Peer.Pid
				if len(tr.Ancestors(selfPid)) > len(tr.Ancestors(peerPid)) ||
					len(tr.Ancestors(selfPid)) == len(tr.Ancestors(peerPid)) && conn.Self.Pid > conn.Peer.Pid {
					selfPid, peerPid = peerPid, selfPid
					self, peer = peer, self
				}
				id := [2]Pid{selfPid, peerPid}
				if _, ok := edgeTooltips[id]; !ok {
					edgeTooltips[id] = map[string]struct{}{}
				}
				edgeTooltips[id][fmt.Sprintf(
					"%s:%s&#10142;%s",
					conn.Type,
					self,
					peer,
				)] = struct{}{}
			}
		}
	}

	itr := process.BuildTree(include)

	// connect the parents to their children
	var parents []Pid
	for depth, pid := range itr.All() {
		if depth == 0 {
			parents = []Pid{pid}
			continue
		} else if depth < len(parents) {
			parents = parents[:depth]
			parents = append(parents, pid)
		} else if depth == len(parents) {
			parents = append(parents, pid)
		}
		id := [2]Pid{parents[depth-1], parents[depth]}
		if _, ok := edgeTooltips[id]; !ok {
			edgeTooltips[id] = map[string]struct{}{}
		}
		edgeTooltips[id][fmt.Sprintf(
			"parent:%s&#10142;%s",
			shortname(tb, id[0]),
			shortname(tb, id[1]),
		)] = struct{}{}
	}

	for i := range itr.DepthTree() {
		prcss[i] = map[Pid]string{}
	}

	for depth, pid := range itr.All() {
		prcss[depth][pid] = fmt.Sprintf(
			`[shape=rect style="rounded,filled" fillcolor=%q height=0.3 width=1 URL="%s://localhost:%d/gomon?pid=\N" label="%s\n\N" tooltip=%q]`,
			color(pid),
			scheme,
			flags.port,
			tb[pid].Id.Name,
			longname(tb, pid),
		)

		for id, tooltip := range edgeTooltips {
			self := id[0]
			peer := id[1]
			if self == pid || self < 0 && peer == pid {
				if len(tooltip) > 0 {
					dir := "both"
					var tts []string
					for tt := range tooltip {
						tts = append(tts, tt)
					}
					slices.SortFunc(tts, func(a, b string) int {
						if strings.HasPrefix(a, "parent") {
							return -1
						} else if strings.HasPrefix(b, "parent") {
							return 1
						} else {
							return cmp.Compare(a, b)
						}
					})
					if peer < 0 || peer >= math.MaxInt32 ||
						len(tooltip) == 1 && strings.HasPrefix(tts[0], "parent") {
						dir = "forward"
					}

					edges[id] = fmt.Sprintf(
						`[dir=%s color=%q tooltip="%s"]`,
						dir,
						color(peer)+";0.5:"+color(self),
						strings.Join(tts, "\n"),
					)
				}
			}
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

	host, pid := cluster(tb, hosts)
	if host != "" {
		clusterNodes = append(
			clusterNodes,
			[2]string{pid.String(), "hosts"},
		)
	}

	var procs string
	for depth := range len(prcss) {
		prcs, pid := cluster(tb, prcss[depth])
		if prcs == "" {
			continue
		}
		procs += fmt.Sprintf(
			"   subgraph processes_%d {cluster=true rank=same label=\"Process depth %[1]d\"\n",
			depth+1,
		)
		procs += prcs
		procs += "   }\n"

		clusterNodes = append(
			clusterNodes,
			[2]string{pid.String(), fmt.Sprintf("processes_%d", depth+1)},
		)
	}

	data, pid := cluster(tb, datas)
	if data != "" {
		clusterNodes = append(
			clusterNodes,
			[2]string{pid.String(), "datas"},
		)
	}

	for i := range len(clusterNodes) - 1 {
		clusterEdges += fmt.Sprintf(
			"  %s -> %s [style=invis ltail=%q lhead=%q]\n",
			clusterNodes[i][0],
			clusterNodes[i+1][0],
			clusterNodes[i][1],
			clusterNodes[i+1][1],
		)
	}

	ids := make([][2]Pid, 0, len(edges))
	for id := range edges {
		ids = append(ids, id)
	}

	slices.SortFunc(ids, func(a, b [2]Pid) int {
		return cmp.Or(
			cmp.Compare(a[0], b[0]),
			cmp.Compare(a[1], b[1]),
		)
	})

	var es string
	for _, id := range ids {
		es += fmt.Sprintf(
			"  %d -> %d %s\n",
			id[0],
			id[1],
			edges[id],
		)
	}

	return dot(`digraph "Gomon Process Connections Nodegraph" {
  stylesheet="/assets/mode.css"
  truecolor=true
  fontname=Helvetica
  fontsize=13.0
  label=` + glabel + `
  labelloc=t
  labeljust=l
  rankdir=LR
  newrank=true
  remincross=false
  nodesep=0.05
  ranksep=2
  node [margin=0 fontname=Helvetica fontsize=11.0
  ]
  subgraph hosts {cluster=true rank=same label="External Connections"
` + host + `
  }
  subgraph processes {cluster=true label=Processes
` + procs + `
  }
  subgraph datas {cluster=true rank=same label="Data Sources"
` + data + `
  }
` + clusterEdges + es + `
}`)
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

// cluster returns list of nodes in cluster and id of first node.
func cluster(tb process.Table, nodes map[Pid]string) (string, Pid) {
	if len(nodes) == 0 {
		return "", 0
	}

	pids := make([]Pid, 0, len(nodes))
	for pid := range nodes {
		pids = append(pids, pid)
	}

	slices.SortFunc(pids, func(a, b Pid) int {
		if a >= 0 && a < math.MaxInt32 { // processes
			if n := cmp.Compare(
				filepath.Base(tb[a].Executable),
				filepath.Base(tb[b].Executable),
			); n != 0 {
				return n
			}
		}
		return cmp.Compare(a, b)
	})

	var ns string
	for _, pid := range pids {
		ns += fmt.Sprintf(
			"    %d %s\n",
			pid,
			nodes[pid],
		)
	}

	return ns, pids[0]
}

// dot calls the Graphviz dot command to render the process NodeGraph as gzipped SVG.
func dot(graphviz string) []byte {
	// first write the graph to a file
	// if cwd, err := os.Getwd(); err == nil {
	// 	if f, err := os.CreateTemp(cwd, "graphviz.*.gv"); err == nil {
	// 		os.Chmod(f.Name(), 0644)
	// 		f.WriteString(graphviz)
	// 		f.Close()
	// 	}
	// }

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

// dot calls Graphviz to render the process NodeGraph as gzipped SVG.
// func dot(graphviz string) []byte {
// 	// first write the graph to a file
// 	if cwd, err := os.Getwd(); err == nil {
// 		if f, err := os.CreateTemp(cwd, "graphviz.*.gv"); err == nil {
// 			os.Chmod(f.Name(), 0644)
// 			f.WriteString(graphviz)
// 			f.Close()
// 		}
// 	}

// 	graph := C.CString(graphviz)
// 	defer C.free(unsafe.Pointer(graph))

// 	gvc := C.gvContext()
// 	defer C.gvFreeContext(gvc)

// 	g := C.agmemread(graph)
// 	defer C.agclose(g)

// 	layout := C.CString("dot")
// 	defer C.free(unsafe.Pointer(layout))
// 	C.gvLayout(gvc, g, layout)
// 	defer C.gvFreeLayout(gvc, g)

// 	format := C.CString("svgz")
// 	defer C.free(unsafe.Pointer(format))
// 	var data *C.char
// 	var length C.uint
// 	rc, err := C.gvRenderData(gvc, g, format, &data, &length)
// 	if rc != 0 {
// 		gocore.Error("dot", err).Err()
// 		return nil
// 	}
// 	buf := C.GoBytes(unsafe.Pointer(data), C.int(length))
// 	C.gvFreeRenderData(data)
// 	return buf
// }
