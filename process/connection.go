// Copyright © 2021-2023 The Gomon Project.

package process

import (
	"fmt"
	"runtime"
	"slices"

	"github.com/zosmac/gocore"
)

// connections determines the remote endpoints of each process' connections.
func connections(epm map[Pid][]Connection) {
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
	eps := map[[3]string][]Pid{}
	for _, conns := range epm {
		for _, conn := range conns {
			if conn.Type == "unix" && conn.Self.Name != "" && conn.Peer.Name[0] == '/' { // named socket
				eps[[3]string{conn.Type, conn.Self.Name, ""}] =
					append(eps[[3]string{conn.Type, conn.Self.Name, ""}], conn.Self.Pid)
			} else if conn.Peer.Pid == 0 {
				eps[[3]string{conn.Type, conn.Self.Name, ""}] =
					append(eps[[3]string{conn.Type, conn.Self.Name, ""}], conn.Self.Pid)
				eps[[3]string{conn.Type, "", conn.Peer.Name}] =
					append(eps[[3]string{conn.Type, "", conn.Peer.Name}], conn.Self.Pid)
			}
		}
	}

	for _, pids := range eps {
		slices.Sort(pids)
	}

	for ep, peers := range eps {
		if ep[2] == "" {
			continue
		}
		if selves, ok := eps[[3]string{ep[0], ep[2], ""}]; ok {
			self := selves[0]
			for _, peer := range peers {
				for i, conn := range epm[peer] {
					if conn.Peer.Name == ep[2] {
						conn.Peer.Pid = self
						epm[peer][i].Peer.Pid = self
						// add connections for dup'd or inherited endpoints
						for _, self := range selves[1:] {
							conn := epm[peer][i]
							conn.Peer.Pid = self
							epm[peer] = append(epm[peer], conn)
						}
					}
				}
			}
		}
	}
}
