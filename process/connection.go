// Copyright © 2021-2023 The Gomon Project.

package process

import (
	"fmt"
	"runtime"
	"sort"

	"github.com/zosmac/gocore"
)

// Connections creates a slice of local to remote connections.
func Connections(tb Table) {
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			buf = buf[:n]
			gocore.Error("connections", fmt.Errorf("%v", r), map[string]string{
				"stacktrace": string(buf),
			}).Err()
		}
	}()

	pids := make([]Pid, 0, len(tb))
	for pid := range tb {
		pids = append(pids, pid)
	}
	sort.Slice(pids, func(i, j int) bool {
		return pids[i] < pids[j]
	})

	// build a map for identifying intra-host peer endpoints
	epm := map[[3]string][]Pid{} // is distinguishing dup'd and inherited descriptors an issue?
	for _, pid := range pids {
		p := tb[pid]
		for _, conn := range p.Connections {
			if conn.Type == "unix" && conn.Self.Name != "" && conn.Peer.Name[0] == '/' { // named socket
				epm[[3]string{conn.Type, conn.Self.Name, ""}] =
					append(epm[[3]string{conn.Type, conn.Self.Name, ""}], conn.Self.Pid)
			} else {
				epm[[3]string{conn.Type, conn.Self.Name, conn.Peer.Name}] =
					append(epm[[3]string{conn.Type, conn.Self.Name, conn.Peer.Name}], conn.Self.Pid)
			}
		}
	}

	for ep, pids := range epm {
		sort.Slice(pids, func(i, j int) bool {
			return pids[i] < pids[j]
		})
		epm[ep] = pids
	}

	for _, pid := range pids {
		p := tb[pid]
		var conns []Connection
		for i, conn := range p.Connections {
			if conn.Peer.Name == "" {
				continue // listener
			}

			if conn.Self.Name == "" {
				continue // data connection
			}

			rpids, ok := epm[[3]string{conn.Type, conn.Peer.Name, conn.Self.Name}]
			if !ok {
				if rpids, ok = epm[[3]string{conn.Type, conn.Peer.Name, ""}]; ok { // partner with unix named socket
					for _, rpid := range rpids {
						for i, cn := range tb[rpid].Connections {
							if cn.Self.Name == conn.Peer.Name {
								tb[rpid].Connections[i].Peer.Name = conn.Self.Name
								tb[rpid].Connections[i].Peer.Pid = pid
							}
						}
					}
				}
			}
			if ok {
				p.Connections[i].Peer.Pid = rpids[0] // intra-process connection
				for _, rpid := range rpids[1:] {
					conn := p.Connections[i]
					conn.Peer.Pid = rpid
					conns = append(conns, conn)
				}
			}
		}
		p.Connections = append(p.Connections, conns...)
	}
}
