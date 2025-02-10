// Copyright © 2021-2023 The Gomon Project.

package process

import (
	"fmt"
	"runtime"
	"slices"

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

	// build a map for identifying intra-host peer endpoints
	epm := map[[3]string][]Pid{} // is distinguishing dup'd and inherited descriptors an issue?
	for _, p := range tb {
		for _, conn := range p.Connections {
			if conn.Type == "unix" && conn.Self.Name != "" && conn.Peer.Name[0] == '/' { // named socket
				epm[[3]string{conn.Type, conn.Self.Name, ""}] =
					append(epm[[3]string{conn.Type, conn.Self.Name, ""}], conn.Self.Pid)
			} else if conn.Peer.Pid == 0 {
				epm[[3]string{conn.Type, conn.Self.Name, ""}] =
					append(epm[[3]string{conn.Type, conn.Self.Name, ""}], conn.Self.Pid)
				epm[[3]string{conn.Type, "", conn.Peer.Name}] =
					append(epm[[3]string{conn.Type, "", conn.Peer.Name}], conn.Self.Pid)
			}
		}
	}

	for ep, pids := range epm {
		if ep[2] == "" {
			slices.SortFunc(pids, func(a, b Pid) int {
				return -tb[a].Starttime.Compare(tb[b].Starttime)
			})
			epm[ep] = pids[:1] // assume newest process is "owner" of connection
		}
	}

	for ep, peers := range epm {
		if ep[2] == "" {
			continue
		}
		if selves, ok := epm[[3]string{ep[0], ep[2], ""}]; ok {
			self := selves[0]
			for _, peer := range peers {
				for i, cns := range tb[peer].Connections {
					if cns.Peer.Name == ep[2] {
						tb[peer].Connections[i].Peer.Pid = self
					}
				}
			}
		}
	}
}
