// Copyright © 2021-2023 The Gomon Project.

package process

import (
	"fmt"
	"math"
	"time"

	"github.com/zosmac/gocore"
)

type (
	Query[I any, E any, R any] interface {
		Pid() Pid
		BuildGraph(Table, Tree, map[Pid]I, map[int]map[Pid]I, map[Pid]I, map[[2]Pid][]E) R
		HostNode(Connection) I
		DataNode(Connection) I
		ProcNode(*Process) I
		HostEdge(Table, Connection) []E
		DataEdge(Table, Connection) []E
		ProcEdge(Table, Pid, Pid) []E
		Arrow() string
	}
)

func Nodegraph[I any, E any, R any](query Query[I, E, R]) R {
	hosts := map[Pid]I{}         // The host (IP) nodes in the leftmost cluster.
	prcss := map[int]map[Pid]I{} // The process nodes in the process clusters.
	datas := map[Pid]I{}         // The file, unix socket, pipe and kernel connections in the rightmost cluster.
	include := Table{}           // Processes that have a connection to include in report.
	edges := map[[2]Pid][]E{}    // Edges connecting host, process, and data nodes.

	tb := BuildTable()
	tr := tb.BuildTree()
	Connections(tb)

	currCPU := map[Pid]time.Duration{}
	for pid, p := range tb {
		currCPU[pid] = p.Total
	}

	queryPid := query.Pid()
	if queryPid != 0 && tb[queryPid] == nil {
		queryPid = 0 // set to default
	}

	gocore.Error("Nodegraph", nil, map[string]string{
		"pid": queryPid.String(),
	}).Info()

	pt := Table{}
	if queryPid > 0 { // build this process' "extended family"
		for _, pid := range tr.Family(queryPid).All() {
			pt[pid] = tb[pid]
		}
		for _, p := range tb {
			for _, conn := range p.Connections {
				if conn.Peer.Pid == queryPid {
					for _, pid := range tr.Ancestors(conn.Self.Pid) {
						pt[pid] = tb[pid]
					}
					pt[conn.Self.Pid] = tb[conn.Self.Pid]
				}
			}
		}
	} else { // only report non-daemon, remote host connected, and cpu consuming processes
		for pid, p := range tb {
			if pcpu, ok := prevCPU[pid]; !ok || pcpu < currCPU[pid] {
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
				conn.Self.Pid == 1 || conn.Peer.Pid == 1 || // ignore launchd process
				conn.Self.Pid == conn.Peer.Pid || // ignore inter-process connections
				queryPid == 0 && conn.Peer.Pid >= math.MaxInt32 || // ignore data connections for the "all process" query
				(queryPid > 0 && queryPid != conn.Self.Pid && // ignore hosts and datas of connected processes
					(conn.Peer.Pid < 0 || conn.Peer.Pid >= math.MaxInt32)) {
				continue
			}

			if conn.Peer.Pid < 0 { // peer is remote host or listener
				if _, ok := hosts[conn.Peer.Pid]; !ok {
					hosts[conn.Peer.Pid] = query.HostNode(conn)
				}

				// flip the source and target to get Host shown to left in node graph
				id := [2]Pid{conn.Peer.Pid, conn.Self.Pid}
				if _, ok := edges[id]; !ok {
					edges[id] = query.HostEdge(tb, conn)
				}
				edges[id] = append(edges[id], any(fmt.Sprintf(
					"%s:%s"+query.Arrow()+"%s[%d]",
					conn.Type,
					conn.Peer.Name,
					conn.Self.Name,
					conn.Self.Pid,
				)).(E))
			} else if conn.Peer.Pid >= math.MaxInt32 { // peer is data
				peer := conn.Type + ":" + conn.Peer.Name

				if _, ok := datas[conn.Peer.Pid]; !ok {
					datas[conn.Peer.Pid] = query.DataNode(conn)
				}

				id := [2]Pid{conn.Self.Pid, conn.Peer.Pid}
				if _, ok := edges[id]; !ok {
					edges[id] = query.DataEdge(tb, conn)
				}
				edges[id] = append(edges[id], any(fmt.Sprintf(
					"%s"+query.Arrow()+"%s",
					tb[conn.Self.Pid].Shortname(),
					peer,
				)).(E))
			} else { // peer is process
				include[conn.Peer.Pid] = tb[conn.Peer.Pid]
				for _, pid := range tr.Ancestors(conn.Peer.Pid) {
					include[pid] = tb[pid] // add ancestor for BuildTree
				}

				// show edge for inter-process connections only once
				self, peer := conn.Self.Name, conn.Peer.Name
				selfPid, peerPid := conn.Self.Pid, conn.Peer.Pid
				if len(tr.Ancestors(selfPid)) > len(tr.Ancestors(peerPid)) ||
					len(tr.Ancestors(selfPid)) == len(tr.Ancestors(peerPid)) && conn.Self.Pid > conn.Peer.Pid {
					selfPid, peerPid = peerPid, selfPid
					self, peer = peer, self
				}
				id := [2]Pid{selfPid, peerPid}
				if _, ok := edges[id]; !ok {
					edges[id] = query.ProcEdge(tb, id[0], id[1])
				}
				edges[id] = append(edges[id], any(fmt.Sprintf(
					"%s:%s[%d]"+query.Arrow()+"%s[%d]",
					conn.Type,
					self,
					selfPid,
					peer,
					peerPid,
				)).(E))
			}
		}
	}

	itr := include.BuildTree()

	// connect the parents to their children
	var parents []Pid
	for depth, pid := range itr.All() {
		if depth == 0 {
			parents = []Pid{pid} // top of the tree
			continue
		} else if depth < len(parents) {
			parents = parents[:depth]      // move back up tree
			parents = append(parents, pid) // and start new branch
		} else if depth == len(parents) {
			parents = append(parents, pid) // descend branch
		}
		id := [2]Pid{parents[depth-1], parents[depth]}
		if _, ok := edges[id]; !ok {
			edges[id] = query.ProcEdge(tb, id[0], id[1])
		}
		edges[id] = append(edges[id], any(fmt.Sprintf(
			"parent:%s"+query.Arrow()+"%s",
			tb[id[0]].Shortname(),
			tb[id[1]].Shortname(),
		)).(E))
	}

	// initialize process clusters
	for i := range itr.DepthTree() - 1 {
		prcss[i] = map[Pid]I{}
	}

	return query.BuildGraph(tb, itr, hosts, prcss, datas, edges)
}

// Longname formats the full Executable name and pid.
func (p *Process) Longname() string {
	if p == nil {
		return ""
	}
	name := p.Executable
	if name == "" {
		name = p.Id.Name
	}
	return fmt.Sprintf("%s[%d]", name, p.Pid)
}

// Shortname formats process name and pid.
func (p *Process) Shortname() string {
	if p == nil {
		return ""
	}
	return fmt.Sprintf("%s[%d]", p.Id.Name, p.Pid)
}
