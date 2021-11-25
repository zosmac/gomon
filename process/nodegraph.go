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
	syslog  bool
	files   bool
}

func parse(r *http.Request) query {
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return query{}
	}
	var pid int
	var kernel, daemons, syslog, files bool
	if v, ok := values["pid"]; ok && len(v) > 0 {
		pid, _ = strconv.Atoi(v[0])
	}
	if v, ok := values["kernel"]; ok && (v[0] == "" || v[0] == "true") {
		kernel = true
	}
	if v, ok := values["daemons"]; ok && (v[0] == "" || v[0] == "true") {
		daemons = true
	}
	if v, ok := values["syslog"]; ok && (v[0] == "" || v[0] == "true") {
		syslog = true
	}
	if v, ok := values["files"]; ok && (v[0] == "" || v[0] == "true") {
		files = true
	}
	return query{
		Pid:     Pid(pid),
		kernel:  kernel || pid > 0,
		daemons: daemons || pid > 0,
		syslog:  syslog || pid > 0,
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
		daemons      string
		processNodes []string
		processEdges []string
		files        string
		fileNode     string
		fileEdges    string
	)
	include := map[Pid]bool{} // record which processes have a connection to include in report, and whether it is a parent

	pt := buildTable()
	q := parse(r)
	if q.Pid > 0 && pt[q.Pid] == nil {
		q = query{} // reset to default
	}
	if q.Pid > 0 {
		ft := processTable{0: pt[0], 1: pt[1], q.Pid: pt[q.Pid]}
		for _, pid := range pt[q.Pid].ancestors {
			ft[pid] = pt[pid]
		}
		ps := flatTree(findTree(buildTree(pt), q.Pid), 0)
		for _, pid := range ps {
			ft[pid] = pt[pid]
		}
		pt = ft
	}

	for _, conn := range connections(pt) {
		var dir string // graphviz arrow direction
		switch conn.direction {
		case "-->>":
			dir = "forward"
		case "<<--":
			dir = "back"
		case "<-->":
			dir = "both"
		case "":
			dir = "none"
		}

		if conn.self.pid == -1 { // external network connections (self.pid/fd = -1/-1)
			if _, ok := include[conn.peer.pid]; !ok {
				include[conn.peer.pid] = false
			}

			host, port, _ := net.SplitHostPort(conn.self.name)
			hosts += fmt.Sprintf(`
    %q [width=1.5 height=0.5 shape=cds label="%s:%s\n%s\n%s"]`,
				conn.self.name,
				conn.ftype,
				port,
				host,
				conn.self.command,
			)
			if hostsNode == "" {
				hostsNode = conn.self.name
			}

			hostEdges += fmt.Sprintf(`
    %q -> "%d:%s" [color="black" dir=both tooltip="%s:%s\n%[2]d:%s"]`,
				conn.self.name,
				conn.peer.pid,
				conn.peer.command,
				conn.ftype,
				conn.name,
			)
		} else if conn.peer.pid == math.MaxInt32 { // peer is file
			if conn.self.pid > 1 && q.files {
				if _, ok := include[conn.self.pid]; !ok {
					include[conn.self.pid] = false
				}

				files += fmt.Sprintf(`
    %q [color="#BBBB99" shape=note label=%q]`,
					conn.name,
					filepath.Base(conn.name),
				)
				if fileNode == "" {
					fileNode = conn.name
				}

				fileEdges += fmt.Sprintf(`
    "%d:%s" -> %q [minlen=4 color="#BBBB99" dir=%s tooltip="%[1]d:%s\n%[5]s:%s"]`,
					conn.self.pid,
					conn.self.command,
					conn.name,
					dir,
					conn.ftype,
					conn.name,
				)
			}
		} else if conn.self.pid == 0 { // ignore kernel
		} else if conn.self.pid == 1 {
			if q.daemons {
				if _, ok := include[conn.peer.pid]; !ok {
					include[conn.peer.pid] = false
				}
			}
		} else if conn.peer.pid == 1 {
			if q.daemons {
				if _, ok := include[conn.self.pid]; !ok {
					include[conn.self.pid] = false
				}
			}
		} else { // peer is process
			if !q.kernel && conn.peer.pid == 0 {
				continue
			}
			if !q.syslog && conn.ftype == "UNIX" && strings.HasSuffix(conn.name, filepath.Join("var", "run", "syslog")) {
				continue
			}

			if conn.self.fd < 0 { // parent/child relationship
				include[conn.self.pid] = true
			}
			if _, ok := include[conn.self.pid]; !ok {
				include[conn.self.pid] = false
			}
			if _, ok := include[conn.peer.pid]; !ok {
				include[conn.peer.pid] = false
			}

			depth := len(pt[conn.self.pid].ancestors)
			for i := len(processNodes); i <= depth; i++ {
				processNodes = append(processNodes, "")
				processEdges = append(processEdges, "")
			}
			if processNodes[depth] == "" {
				processNodes[depth] = strconv.Itoa(int(conn.self.pid)) + ":" + conn.self.command
			}

			color := color(conn.self.pid)
			if conn.self.fd < 0 { // parent-child connection
				color = "black"
			}

			processEdges[depth] += fmt.Sprintf(`
      "%d:%s" -> "%d:%s" [color=%q dir=%s tooltip="%s:%s\n%[1]d:%s\n%d:%s"]`,
				conn.self.pid,
				conn.self.command,
				conn.peer.pid,
				conn.peer.command,
				color,
				dir,
				conn.ftype,
				conn.name,
			)
		}
	}

	delete(include, 0) // remove process 0
	// for _, pid := range flatTree(buildTree(pt), 0) {
	for pid, p := range pt {
		parent, ok := include[pid]
		if !ok {
			continue
		}
		// p := pt[pid]

		for i := len(processes); i <= len(p.ancestors); i++ {
			processes = append(processes, fmt.Sprintf(`
    subgraph cluster_processes_%d {
      label="Process depth %[1]d" rank=same fontsize=11 penwidth=3.0 pencolor="#5599BB"`,
				i+1))
		}
		node := fmt.Sprintf(`
      "%d:%s" [color=%q label=%q URL="http://localhost:%d/gomon?pid=%[1]d"]`,
			pid,
			p.Exec,
			color(pid),
			p.Id.Name,
			core.Flags.Port,
		)
		if !parent && len(p.ancestors) == 0 {
			daemons += node
		} else {
			processes[len(p.ancestors)] += node
		}
	}
	processes[0] += daemons
	for i := range processes {
		processes[i] += "\n    }"
	}

	if len(processNodes) > 0 {
		if hostsNode != "" {
			clusterEdges += fmt.Sprintf(`
  %q -> %q [style=invis ltail="cluster_hosts" lhead="cluster_processes_1"]`,
				hostsNode,
				processNodes[0],
			)
		}
		for i := range processNodes[:len(processNodes)-1] {
			clusterEdges += fmt.Sprintf(`
  %q -> %q [style=invis ltail="cluster_processes_%d" lhead="cluster_processes_%d"]`,
				processNodes[i],
				processNodes[i+1],
				i+1,
				i+2,
			)
		}
		if q.files {
			clusterEdges += fmt.Sprintf(`
  %q -> %q [style=invis ltail="cluster_processes_%d" lhead="cluster_files"]`,
				processNodes[len(processNodes)-1],
				fileNode,
				len(processNodes),
			)
		}
	}

	label := "Remote Hosts and Inter-Process Connections for " +
		core.HostName + " on " +
		time.Now().Local().Format("Mon Jan 02 2006, at 03:04:05PM MST")

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
  }` +
		hostEdges + `
  subgraph cluster_processes {
    label=Processes fontsize=11 penwidth=3.0 pencolor="#5599BB"` +
		strings.Join(processes, "") +
		strings.Join(processEdges, "") + `
  }
  subgraph cluster_files {
    label="Open Files" rank=max fontsize=11 penwidth=3 pencolor="#99BB55"` +
		files +
		fileEdges + `
  }` +
		clusterEdges + `
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
		core.LogError(fmt.Errorf("dot command failed %v %s", err, stderr.Bytes()))
		sc := bufio.NewScanner(strings.NewReader(graphviz))
		for i := 1; sc.Scan(); i++ {
			fmt.Fprintf(os.Stderr, "%4.d %s\n", i, sc.Text())
		}
		return nil
	}

	return stdout.Bytes()
}
