// Copyright © 2021-2023 The Gomon Project.

package serve

/*
#cgo CFLAGS: -I/usr/local/include
#cgo LDFLAGS: -L/usr/local/lib -lgvc -lcgraph

#include <graphviz/gvc.h>
#include <stdlib.h>
*/
import "C"

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
	"slices"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/zosmac/gocore"
	"github.com/zosmac/gomon/process"
)

type (
	// Pid alias for Pid in process package.
	Pid = process.Pid

	// query parameters for request.
	Query struct {
		pid Pid
	}
)

const (
	// use_dot_command specifies whether nodegraph rendering uses the graphviz API or the dot command.
	use_dot_command = false

	// output_format specifies the output file format for graphviz to generate.
	output_format = "svg" // unfortunately, using "svgz" often produces "Error: deflation finish problem 0 cnt=102"
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
	color = colors[(int(pid))%len(colors)]
	return color
}

// Nodegraph produces the process connections node graph.
func Nodegraph(req *http.Request) []byte {
	return process.Nodegraph[string, string, []byte](parseQuery(req))
}

// Pid returns the query's pid.
func (query Query) Pid() Pid {
	return query.pid
}

// Arrow returns the character to use in edges' tooltip connections list.
func (query Query) Arrow() string {
	return " &#10142; "
}

func (query Query) BuildGraph(
	tb process.Table,
	itr process.Tree,
	hosts map[Pid]string,
	prcss map[int]map[Pid]string,
	datas map[Pid]string,
	edges map[[2]Pid][]string,
) []byte {
	var clusterNodes [][2]string // One node in each cluster is linked invisibly to a node
	var clusterEdges string      //  in the next to maintain left to right order of clusters.

	// add process nodes to each cluster, sort connections for tooltip
	for depth, pid := range itr.All() {
		prcss[depth][pid] = query.ProcNode(tb[pid])
		for id, edge := range edges {
			self := id[0]
			peer := id[1]
			if self == pid || self < 0 && peer == pid {
				slices.SortFunc(edge[1:], func(a, b string) int { // tooltips list edge's connection endpoints
					if strings.HasPrefix(a, "parent") {
						return -1
					} else if strings.HasPrefix(b, "parent") {
						return 1
					} else {
						return cmp.Compare(a, b)
					}
				})
			}
		}
	}

	// prepare label for nodegraph
	var pslabel string
	if query.Pid() > 0 {
		pslabel = " Process: " + tb[query.Pid()].Shortname()
	}

	glabel := fmt.Sprintf(
		`"External and Inter-Process Connections\lHost: %s%s%s`,
		gocore.Host,
		pslabel,
		time.Now().Local().Format(`\lMon Jan 02 2006 at 03:04:05PM MST\l"`),
	)

	// build hosts cluster
	host, pid := cluster(tb, hosts)
	if host != "" {
		clusterNodes = append(
			clusterNodes,
			[2]string{pid.String(), "hosts"},
		)
	}

	// build processes clusters
	var procs string
	for depth := range len(prcss) {
		prcs, pid := cluster(tb, prcss[depth])
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

	// build datas cluster (files, sockets, pipes, ...)
	data, pid := cluster(tb, datas)
	if data != "" {
		clusterNodes = append(
			clusterNodes,
			[2]string{pid.String(), "datas"},
		)
	}

	// define invisible edges to force left to right positioning of hosts -> processes -> data clusters
	for i := range len(clusterNodes) - 1 {
		clusterEdges += fmt.Sprintf(
			"  %s -> %s [style=invis ltail=%q lhead=%q]\n",
			clusterNodes[i][0],
			clusterNodes[i+1][0],
			clusterNodes[i][1],
			clusterNodes[i+1][1],
		)
	}

	// add the edges
	// var ids [][2]Pid
	// for id := range edges {
	// 	ids = append(ids, id)
	// }

	// slices.SortFunc(ids, func(a, b [2]Pid) int {
	// 	return cmp.Or(
	// 		cmp.Compare(a[0], b[0]),
	// 		cmp.Compare(a[1], b[1]),
	// 	)
	// })

	var es string
	for id := range edges {
		dir := "both"
		if id[1] >= 0 && id[1] < math.MaxInt32 && tb[id[1]].Ppid == id[0] {
			dir = "forward"
		}
		es += fmt.Sprintf("%s dir=%s tooltip=%q]", edges[id][0], dir, strings.Join(edges[id][1:], "\n"))
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
func parseQuery(r *http.Request) *Query {
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		gocore.Error("parseQuery", err).Err()
		return nil
	}
	var pid int
	if v, ok := values["pid"]; ok && len(v) > 0 {
		pid, _ = strconv.Atoi(v[0])
	}
	return &Query{
		pid: Pid(pid),
	}
}

func (query Query) HostNode(conn process.Connection) string {
	host, port, _ := net.SplitHostPort(conn.Peer.Name)
	return fmt.Sprintf(
		`[shape=cds style=filled fillcolor=%q height=0.6 width=2 label="%s:%s\n%s" tooltip=%q]`,
		color(conn.Peer.Pid),
		conn.Type,
		port,
		gocore.Hostname(host),
		conn.Peer.Name,
	)
}

func (query Query) DataNode(conn process.Connection) string {
	return fmt.Sprintf(
		`[shape=note style=filled fillcolor=%q height=0.2 label=%q tooltip=%q]`,
		color(conn.Peer.Pid),
		conn.Type+":"+conn.Peer.Name,
		conn.Peer.Name,
	)
}

func (query Query) ProcNode(p *process.Process) string {
	return fmt.Sprintf(
		`[shape=rect style="rounded,filled" fillcolor=%q height=0.3 width=1 URL="%s://localhost:%d/gomon?pid=\N" label="%s\n\N" tooltip=%q]`,
		color(p.Pid),
		scheme,
		flags.port,
		p.Id.Name,
		p.Longname(),
	)
}

func (query Query) HostEdge(_ process.Table, conn process.Connection) []string {
	return []string{fmt.Sprintf(
		`  %d -> %d [color=%q`,
		conn.Peer.Pid,
		conn.Self.Pid,
		color(conn.Self.Pid)+";0.5:"+color(conn.Peer.Pid),
	)}
}

func (query Query) DataEdge(_ process.Table, conn process.Connection) []string {
	return []string{fmt.Sprintf(
		`  %d -> %d [color=%q`,
		conn.Self.Pid,
		conn.Peer.Pid,
		color(conn.Peer.Pid)+";0.5:"+color(conn.Self.Pid),
	)}
}

func (query Query) ProcEdge(_ process.Table, self, peer Pid) []string {
	return []string{fmt.Sprintf(
		`  %d -> %d [color=%q`,
		self,
		peer,
		color(peer)+";0.5:"+color(self),
	)}
}

// cluster returns list of nodes in cluster and id of first node.
func cluster(_ process.Table, nodes map[Pid]string) (string, Pid) {
	if len(nodes) == 0 {
		return "", 0
	}

	// var pids []Pid
	// for pid := range nodes {
	// 	pids = append(pids, pid)
	// }

	// slices.SortFunc(pids, func(a, b Pid) int {
	// 	if a >= 0 && a < math.MaxInt32 { // processes
	// 		if n := cmp.Compare(
	// 			filepath.Base(tb[a].Executable),
	// 			filepath.Base(tb[b].Executable),
	// 		); n != 0 {
	// 			return n
	// 		}
	// 	}
	// 	return cmp.Compare(a, b)
	// })

	// invis := pids[0]
	// var ns string
	// for _, pid := range pids {
	// 	ns += fmt.Sprintf("    %d %s\n", pid, nodes[pid])
	// }

	var invis Pid
	var ns string
	for pid, node := range nodes {
		invis = pid
		ns += fmt.Sprintf("    %d %s\n", pid, node)
	}

	return ns, invis
}

// dot calls the Graphviz dot command to render the process NodeGraph as SVG.
func dot(graphviz string) (buf []byte) {
	// // to debug the graph, write to a file
	// if cwd, err := os.Getwd(); err == nil {
	// 	if f, err := os.CreateTemp(cwd, "graphviz.*.gv"); err == nil {
	// 		os.Chmod(f.Name(), 0644)
	// 		f.WriteString(graphviz)
	// 		f.Close()
	// 	}
	// }

	if use_dot_command {
		cmd := exec.Command("dot", "-v", "-T"+output_format)
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
			return
		}
		buf = stdout.Bytes()
	} else {
		graph := C.CString(graphviz)
		defer C.free(unsafe.Pointer(graph))

		gvc := C.gvContext()
		defer C.gvFreeContext(gvc)

		g := C.agmemread(graph)
		defer C.agclose(g)

		layout := C.CString("dot")
		defer C.free(unsafe.Pointer(layout))
		C.gvLayout(gvc, g, layout)
		defer C.gvFreeLayout(gvc, g)

		format := C.CString(output_format)
		defer C.free(unsafe.Pointer(format))
		var data *C.char
		var length C.uint
		rc, err := C.gvRenderData(gvc, g, format, &data, &length)
		if rc != 0 {
			gocore.Error("dot", err).Err()
			return
		}
		buf = C.GoBytes(unsafe.Pointer(data), C.int(length))
		C.gvFreeRenderData(data)
	}
	return
}
