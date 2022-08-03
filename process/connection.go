// Copyright © 2021 The Gomon Project.

package process

import (
	"fmt"
	"runtime"

	"github.com/zosmac/gomon/core"
)

// Connections creates a slice of local to remote connections.
func Connections(pt Table) {
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			buf = buf[:n]
			core.LogError(fmt.Errorf("Connections() panicked, %v\n%s", r, buf))
		}
	}()

	epm := map[[3]string]Pid{} // is distinguishing dup'd and inherited descriptors an issue?

	// build a map for identifying intra-host peer endpoints
	for _, p := range pt {
		for _, conn := range p.Connections {
			if conn.Type == "unix" && conn.Self.Name != "" && conn.Peer.Name[0] == '/' { // named socket
				epm[[3]string{conn.Type, conn.Self.Name, ""}] = conn.Self.Pid
			} else {
				epm[[3]string{conn.Type, conn.Self.Name, conn.Peer.Name}] = conn.Self.Pid
			}
		}
	}

	hdpid := Pid(0) // -hdpid for host "pid", hdpid + math.MaxInt32 for data "pid"
	for _, p := range pt {
		pid := p.Pid
		for i, conn := range p.Connections {
			hdpid++

			if conn.Peer.Name == "" {
				continue // listener
			}

			if conn.Self.Name == "" {
				continue // data connection
			}

			rpid, ok := epm[[3]string{conn.Type, conn.Peer.Name, conn.Self.Name}]
			if !ok {
				if rpid, ok = epm[[3]string{conn.Type, conn.Peer.Name, ""}]; ok { // partner with unix named socket
					for i, cn := range pt[rpid].Connections {
						if cn.Self.Name == conn.Peer.Name {
							pt[rpid].Connections[i].Peer.Name = conn.Self.Name
							pt[rpid].Connections[i].Peer.Pid = pid
						}
					}
				}
			}
			if ok {
				p.Connections[i].Peer.Pid = rpid // intra-process connection
			}
		}
		if p.Ppid > 0 {
			p.Connections = append([]Connection{
				{
					Type: "parent",
					Self: Endpoint{
						Name: pt[p.Ppid].Id.Name,
						Pid:  p.Ppid,
					},
					Peer: Endpoint{
						Name: p.Id.Name,
						Pid:  p.Pid,
					},
				}},
				p.Connections...,
			)
		}
	}
}
