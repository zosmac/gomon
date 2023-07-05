// Copyright © 2021-2023 The Gomon Project.

package serve

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
	"sort"
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
		hosts        []string
		prcss        [][]string
		datas        []string
		edges        []string
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

	em := map[string]map[string]struct{}{}

	pids := make([]Pid, 0, len(pt))
	for pid := range pt {
		pids = append(pids, pid)
	}
	sort.Slice(pids, func(i, j int) bool {
		return pids[i] < pids[j]
	})

	for _, pid := range pids {
		p := pt[pid]
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

				if _, ok := nodes[conn.Peer.Pid]; !ok {
					nodes[conn.Peer.Pid] = struct{}{}
					hosts = append(hosts, fmt.Sprintf(
						`%d [shape=cds style=filled fillcolor=%q height=0.6 width=2 label="%s:%s\n%s" tooltip=%q]`,
						conn.Peer.Pid,
						color(conn.Peer.Pid),
						conn.Type,
						port,
						gocore.Hostname(host),
						conn.Peer.Name,
					))
				}

				id := fmt.Sprintf("%d -> %d", conn.Self.Pid, conn.Peer.Pid)
				if _, ok := em[id]; !ok {
					em[id] = map[string]struct{}{}
				}
				em[id][fmt.Sprintf(
					"%s:%s&#10142;%s",
					conn.Type,
					conn.Peer.Name,
					conn.Self.Name,
				)] = struct{}{}
			} else if conn.Peer.Pid >= math.MaxInt32 { // peer is data
				peer := conn.Type + ":" + conn.Peer.Name

				datas = append(datas, fmt.Sprintf(
					`%d [shape=note style=filled fillcolor=%q height=0.2 label=%q tooltip=%q]`,
					conn.Peer.Pid,
					color(conn.Peer.Pid),
					peer,
					conn.Peer.Name,
				))

				// show edge for data connections only once
				id := fmt.Sprintf("%d -> %d", conn.Self.Pid, conn.Peer.Pid)
				if _, ok := em[id]; !ok {
					em[id] = map[string]struct{}{}
				}
				em[id][fmt.Sprintf(
					"%s\n%s",
					shortname(tb, conn.Self.Pid),
					peer,
				)] = struct{}{}
			} else { // peer is process
				include[conn.Peer.Pid] = struct{}{}
				pids := append(conn.Peer.Pid.Ancestors(tb), conn.Peer.Pid)
				for i := range pids[:len(pids)-1] { // add "in-laws"
					include[pids[i]] = struct{}{}
					id := fmt.Sprintf("%d -> %d", pids[i], pids[i+1])
					if _, ok := em[id]; !ok {
						em[id] = map[string]struct{}{}
					}
					em[id][fmt.Sprintf(
						"parent:%s&#10142;%s\n",
						shortname(tb, pids[i]),
						shortname(tb, pids[i+1]),
					)] = struct{}{}
				}

				// show edge for inter-process connections only once
				self, peer := conn.Self.Name, conn.Peer.Name
				selfPid, peerPid := conn.Self.Pid, conn.Peer.Pid
				if len(selfPid.Ancestors(tb)) > len(peerPid.Ancestors(tb)) ||
					len(selfPid.Ancestors(tb)) == len(peerPid.Ancestors(tb)) && conn.Self.Pid > conn.Peer.Pid {
					selfPid, peerPid = peerPid, selfPid
					self, peer = peer, self
				}
				id := fmt.Sprintf("%d -> %d", selfPid, peerPid)
				if _, ok := em[id]; !ok {
					em[id] = map[string]struct{}{}
				}
				em[id][fmt.Sprintf("%s:%s&#10142;%s\n", // non-breaking space/hyphen
					conn.Type,
					self,
					peer,
				)] = struct{}{}
			}
		}
	}

	pids = make([]Pid, 0, len(tb))
	for pid := range tb {
		pids = append(pids, pid)
	}
	sort.Slice(pids, func(i, j int) bool {
		return pids[i] < pids[j]
	})

	for _, pid := range pids {
		if _, ok := include[pid]; !ok {
			continue
		}

		depth := len(pid.Ancestors(tb))
		for i := len(prcss); i <= depth; i++ {
			prcss = append(prcss, []string{})
		}

		prcss[depth] = append(prcss[depth], fmt.Sprintf(
			`%d [shape=rect style="rounded,filled" fillcolor=%q height=0.3 width=1 URL="%s://localhost:%d/gomon?pid=\N" label="%s\n\N" tooltip=%q]`,
			pid,
			color(pid),
			scheme,
			flags.port,
			tb[pid].Id.Name,
			longname(tb, pid),
		))

		for edge, tooltip := range em {
			fields := strings.Fields(edge)
			self, _ := strconv.Atoi(fields[0])
			peer, _ := strconv.Atoi(fields[2])
			if Pid(self) == pid {
				if len(tooltip) > 0 {
					dir := "both"
					var tts string
					for tt := range tooltip {
						if strings.HasPrefix(tt, "parent") {
							tts = tt + tts
						} else {
							tts += tt
						}
					}
					if peer < 0 || peer >= math.MaxInt32 ||
						len(tooltip) == 1 && strings.HasPrefix(tts, "parent") {
						dir = "forward"
					}

					edges = append(edges, fmt.Sprintf(
						`%s [dir=%s color=%q tooltip="%s"]`,
						edge,
						dir,
						color(Pid(peer))+";0.5:"+color(Pid(self)),
						tts,
					))
				}
				delete(em, edge)
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

	sort.Slice(hosts, func(i, j int) bool {
		a, _ := strconv.Atoi(hosts[i][:strings.Index(hosts[i], " ")])
		b, _ := strconv.Atoi(hosts[j][:strings.Index(hosts[j], " ")])
		return a < b
	})

	var clusterNodes []string
	if len(hosts) > 0 {
		clusterNodes = append(
			clusterNodes,
			hosts[0][:strings.Index(hosts[0], " ")],
			"hosts",
		)
	}

	var procs []string
	for i, prcs := range prcss {
		if len(prcs) > 0 {
			sort.Slice(prcs, func(i, j int) bool {
				a, _ := strconv.Atoi(prcs[i][:strings.Index(prcs[i], " ")])
				b, _ := strconv.Atoi(prcs[j][:strings.Index(prcs[j], " ")])
				return a < b
			})
			clusterNodes = append(
				clusterNodes,
				prcs[0][:strings.Index(prcs[0], " ")],
				fmt.Sprintf("processes_%d", i+1),
			)

			procs = append(procs, fmt.Sprintf(`subgraph processes_%d {cluster=true rank=same label="Process depth %[1]d"`, i+1))
			procs = append(procs, prcs...)
			procs = append(procs, "}")
		}
	}

	sort.Slice(datas, func(i, j int) bool {
		a, _ := strconv.Atoi(datas[i][:strings.Index(datas[i], " ")])
		b, _ := strconv.Atoi(datas[j][:strings.Index(datas[j], " ")])
		return a < b
	})

	if len(datas) > 0 {
		clusterNodes = append(
			clusterNodes,
			datas[0][:strings.Index(datas[0], " ")],
			"datas",
		)
	}

	for i := range clusterNodes[:len(clusterNodes)-2] {
		if i%2 == 0 {
			clusterEdges += fmt.Sprintf(
				"  %s -> %s [style=invis ltail=%q lhead=%q]\n",
				clusterNodes[i],
				clusterNodes[i+2],
				clusterNodes[i+1],
				clusterNodes[i+3],
			)
		}
	}

	sort.Slice(edges, func(i, j int) bool {
		s := strings.SplitN(edges[i], " ", 4)
		t := strings.SplitN(edges[j], " ", 4)
		a, _ := strconv.Atoi(s[0])
		b, _ := strconv.Atoi(t[0])
		c, _ := strconv.Atoi(s[2])
		if c < 0 {
			a, c = c, a
		}
		d, _ := strconv.Atoi(t[2])
		if d < 0 {
			b, d = d, b
		}
		return a < b ||
			a == b && c < d
	})

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
  remincross=false
  nodesep=0.03
  ranksep=2
  node [margin=0]
  subgraph hosts {cluster=true rank=source label="External Connections"
    ` + strings.Join(hosts, "\n    ") + `
  }
  subgraph processes {cluster=true label=Processes
      ` + strings.Join(procs, "\n      ") + `
  }
  subgraph datas {cluster=true rank=sink label="Open Files"
    ` + strings.Join(datas, "\n    ") + `
  }
` + clusterEdges + strings.Join(edges, "\n  ") + `
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
	pids := append(pid.Ancestors(tb), pid)
	for _, pid := range pids { // ancestors
		pt[pid] = tb[pid]
	}

	tr = tr.FindTree(pid)
	o := func(node Pid, pt process.Table) int {
		return order(node, tr, pt)
	}

	for _, pid := range tr.Flatten(tb, o) {
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
