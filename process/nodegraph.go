// Copyright © 2021 The Gomon Project.

package process

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
	"strconv"
	"strings"
	"time"

	"github.com/zosmac/gomon/core"
)

var (
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

type query struct {
	Pid
	kernel  bool
	daemons bool
	files   bool
}

func parse(r *http.Request) query {
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return query{}
	}
	var pid int
	var kernel, daemons, files bool
	if v, ok := values["pid"]; ok && len(v) > 0 {
		pid, _ = strconv.Atoi(v[0])
	}
	if v, ok := values["kernel"]; ok && (v[0] == "" || v[0] == "true") {
		kernel = true
	}
	if v, ok := values["daemons"]; ok && (v[0] == "" || v[0] == "true") {
		daemons = true
	}
	if v, ok := values["files"]; ok && (v[0] == "" || v[0] == "true") {
		files = true
	}
	return query{
		Pid:     Pid(pid),
		kernel:  kernel || pid > 0,
		daemons: daemons || pid > 0,
		files:   files || pid > 0,
	}
}

// NodeGraph returns the process connections node graph.
func NodeGraph(r *http.Request) []byte {
	var (
		clusterEdges string
		hosts        string
		hostsNode    string
		hostEdges    string
		processes    []string
		processNodes []string
		processEdges []string
		files        string
		fileNode     string
		fileEdges    string
		include      = map[Pid]struct{}{} // record which processes have a connection to include in report
	)

	pt := buildTable()
	conns := connections(pt)

	q := parse(r)
	if q.Pid > 0 && pt[q.Pid] == nil {
		q = query{} // reset to default
	}
	if q.Pid > 0 {
		ft := map[Pid]struct{}{q.Pid: {}}
		for _, pid := range pt[q.Pid].ancestors {
			ft[pid] = struct{}{}
		}
		ps := flatTree(findTree(buildTree(pt), q.Pid), 0) // descendants
		for _, pid := range ps {
			ft[pid] = struct{}{}
		}
		var cs []connection
		for _, conn := range conns {
			if _, ok := ft[conn.self.pid]; ok {
				cs = append(cs, conn)
			} else if _, ok := ft[conn.peer.pid]; ok {
				cs = append(cs, conn)
			}
		}
		conns = cs
	}

	if q.kernel {
		hosts = `
    0 [width=1.0 height=0.2 label="kernel"]`
		hostsNode = "0"
	}

	for _, conn := range conns {
		var dir string // graphviz arrow direction
		switch conn.direction {
		case "-->>":
			dir = "forward"
		case "<<--":
			dir = "back"
		case "<-->":
			dir = "both"
		default:
			dir = "none"
		}

		if conn.self.pid == -1 { // external network connections (self.pid/fd = -1/-1)
			include[conn.peer.pid] = struct{}{}

			host, port, _ := net.SplitHostPort(conn.self.name)
			hosts += fmt.Sprintf(`
    %q [width=1.5 height=0.5 shape=cds label="%s:%s\n%s"]`,
				conn.self.name,
				conn.ftype,
				port,
				hostname(host),
			)
			if hostsNode == "" {
				hostsNode = conn.self.name
			}

			host, _, _ = net.SplitHostPort(conn.peer.name)
			hostEdges += fmt.Sprintf(`
  %q -> %d [color="black" dir=both tooltip="%s:%s\n%[2]d:%[5]s"]`,
				conn.self.name,
				conn.peer.pid,
				interfaces[host],
				conn.name,
				pt[conn.peer.pid].Exec,
			)
		} else if conn.peer.pid == math.MaxInt32 { // peer is file, add node after all processes identified
		} else if conn.self.pid == 0 { // ignore kernel
		} else if conn.self.pid == 1 {
			if q.daemons {
				include[conn.peer.pid] = struct{}{}
			}
		} else if conn.peer.pid == 1 {
			if q.daemons {
				include[conn.self.pid] = struct{}{}
			}
		} else { // peer is process
			var peerExec string
			if conn.peer.pid == 0 {
				if !q.kernel {
					continue
				}
				peerExec = "kernel"
			} else {
				peerExec = filepath.Base(pt[conn.peer.pid].Exec)
			}

			include[conn.self.pid] = struct{}{}
			include[conn.peer.pid] = struct{}{}

			depth := len(pt[conn.self.pid].ancestors)
			for i := len(processNodes); i <= depth; i++ {
				processNodes = append(processNodes, "")
				processEdges = append(processEdges, "")
			}
			if processNodes[depth] == "" {
				processNodes[depth] = conn.self.pid.String()
			}

			color := color(conn.self.pid)
			if strings.HasPrefix(conn.ftype, "parent:") {
				color = "black"
			}

			t := conn.ftype
			if t == "TCP" || t == "UDP" {
				ip, _, _ := net.SplitHostPort(conn.self.name)
				t = interfaces[ip]
			}

			processEdges[depth] += fmt.Sprintf(`
  %d -> %d [color=%q dir=%s tooltip="%s:%s\n%[1]d:%[7]s\n%[2]d:%[8]s"]`,
				conn.self.pid,
				conn.peer.pid,
				color,
				dir,
				t,
				conn.name,
				pt[conn.self.pid].Exec,
				peerExec,
			)
		}
	}

	delete(include, 0) // remove process 0
	for _, pid := range flatTree(buildTree(pt), 0) {
		// for pid, p := range pt {
		if _, ok := include[pid]; !ok {
			continue
		}
		p := pt[pid]

		for i := len(processes); i <= len(p.ancestors); i++ {
			processes = append(processes, fmt.Sprintf(`
    subgraph cluster_processes_%d {
      label="Process depth %[1]d" rank=same fontsize=11 penwidth=3.0 pencolor="#5599BB"`,
				i+1))
		}

		node := fmt.Sprintf(`
      %[2]d [color=%q label=%q tooltip="%s\n[%[2]d]" URL="http://localhost:%[1]d/gomon?pid=%d"]`,
			core.Flags.Port,
			pid,
			color(pid),
			p.Id.Name,
			p.Exec,
		)
		processes[len(p.ancestors)] += node
	}
	for i := range processes {
		processes[i] += "\n    }"
	}

	if q.files {
		for _, conn := range conns {
			if conn.peer.pid == math.MaxInt32 { // peer is file
				if _, ok := include[conn.self.pid]; !ok {
					continue
				}

				var dir string // graphviz arrow direction
				switch conn.direction {
				case "-->>":
					dir = "forward"
				case "<<--":
					dir = "back"
				case "<-->":
					dir = "both"
				default:
					dir = "none"
				}

				var color, label string
				switch conn.ftype {
				case "DIR":
					color = "#00FF00"
					label = conn.name + string(filepath.Separator)
				case "REG":
					color = "#BBBB99"
					label = filepath.Base(conn.name)
				case "PSXSHM":
					color = "#FF0000"
					label = conn.name
				}

				files += fmt.Sprintf(`
    %q [color=%q shape=note label=%q tooltip=%[1]q]`,
					conn.name,
					color,
					label,
				)
				if fileNode == "" {
					fileNode = conn.name
				}

				fileEdges += fmt.Sprintf(`
  %d -> %q [minlen=4 color=%q dir=%s tooltip="%[1]d:%[5]s\n%s:%[2]s"]`,
					conn.self.pid,
					conn.name,
					color,
					dir,
					pt[conn.self.pid].Exec,
					conn.ftype,
				)
			}
		}
	}

	if len(processNodes) > 0 {
		if hostsNode != "" {
			clusterEdges += fmt.Sprintf(`
  %q -> %s [style=invis ltail="cluster_hosts" lhead="cluster_processes_1"]`,
				hostsNode,
				processNodes[0],
			)
		}
		for i := range processNodes[:len(processNodes)-1] {
			clusterEdges += fmt.Sprintf(`
  %s -> %s [style=invis ltail="cluster_processes_%d" lhead="cluster_processes_%d"]`,
				processNodes[i],
				processNodes[i+1],
				i+1,
				i+2,
			)
		}
		if q.files {
			clusterEdges += fmt.Sprintf(`
  %s -> %q [style=invis ltail="cluster_processes_%d" lhead="cluster_files"]`,
				processNodes[len(processNodes)-1],
				fileNode,
				len(processNodes),
			)
		}
	}

	var label string
	if q.Pid > 0 {
		label = fmt.Sprintf("Inter-Process Connections for Process %s[%d] on ", filepath.Base(pt[q.Pid].Exec), q.Pid)
	} else {
		label = "Remote Hosts and Inter-Process Connections for "
	}
	label += core.HostName + time.Now().Local().Format(", Mon Jan 02 2006 at 03:04:05PM MST")

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
  ranksep=2.0
  node [shape=rect fontname=helvetica fontsize=7 width=1 height=0.1 style=rounded]
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
    label="Open Files" rank=max fontsize=11 penwidth=3 pencolor="#99BB55"` +
		files + `
  }` +
		clusterEdges +
		hostEdges +
		strings.Join(processEdges, "") +
		fileEdges + `
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
