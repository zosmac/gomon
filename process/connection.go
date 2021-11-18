// Copyright © 2021 The Gomon Project.

package process

import (
	"fmt"
	"math"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
)

var (
	// ipinfo defines message format for ip interface info
	ipinfo = `  unicast:
    global:                %v
    link local:            %v
  multicast:               %v
    interface local:       %v
    link local:            %v
  loopback:                %v
  unspecified:             %v
`
	// localIps addresses for all local network interfaces on host
	localIps = func() map[string]struct{} {
		h, _ := os.Hostname()
		ips, _ := net.LookupIP(h)
		l := map[string]struct{}{}
		for _, ip := range ips {
			l[ip.String()] = struct{}{}
			// fmt.Fprintf(os.Stderr,
			// 	"local IP: %s\n"+ipinfo,
			// 	ip.String(),
			// 	ip.IsGlobalUnicast(),
			// 	ip.IsLinkLocalUnicast(),
			// 	ip.IsMulticast(),
			// 	ip.IsInterfaceLocalMulticast(),
			// 	ip.IsLinkLocalMulticast(),
			// 	ip.IsLoopback(),
			// 	ip.IsUnspecified(),
			// )
		}
		return l
	}()

	// remoteHosts caches DNS lookup hostnames
	remoteHosts = map[string]string{}

	// interfaces maps local ip addresses to their network interfaces.
	interfaces = func() map[string]string {
		im := map[string]string{}
		if nis, err := net.Interfaces(); err == nil {
			for _, ni := range nis {
				if addrs, err := ni.Addrs(); err == nil {
					for _, addr := range addrs {
						if ip, _, err := net.ParseCIDR(addr.String()); err == nil {
							im[ip.String()] = ni.Name
							// fmt.Fprintf(os.Stderr,
							// 	"interface %s: %s\n"+ipinfo,
							// 	ni.Name,
							// 	ip.String(),
							// 	ip.IsGlobalUnicast(),
							// 	ip.IsLinkLocalUnicast(),
							// 	ip.IsMulticast(),
							// 	ip.IsInterfaceLocalMulticast(),
							// 	ip.IsLinkLocalMulticast(),
							// 	ip.IsLoopback(),
							// 	ip.IsUnspecified(),
							// )
						}
					}
				}
				if addrs, err := ni.MulticastAddrs(); err == nil {
					for _, addr := range addrs {
						if ip, _, err := net.ParseCIDR(addr.String()); err == nil {
							im[ip.String()] = ni.Name
							// fmt.Fprintf(os.Stderr,
							// 	"interface %s: %s\n"+ipinfo,
							// 	ni.Name,
							// 	ip.String(),
							// 	ip.IsGlobalUnicast(),
							// 	ip.IsLinkLocalUnicast(),
							// 	ip.IsMulticast(),
							// 	ip.IsInterfaceLocalMulticast(),
							// 	ip.IsLinkLocalMulticast(),
							// 	ip.IsLoopback(),
							// 	ip.IsUnspecified(),
							// )
						}
					}
				}
			}
		}
		return im
	}()
)

type (
	endpoint struct {
		pid     Pid
		fd      int
		command string
		name    string
	}

	connection struct {
		ftype     string
		name      string
		direction string
		self      endpoint
		peer      endpoint
	}
)

// connections creates an ordered slice of local to remote connections by pid and fd.
func connections(pt processTable) []connection {
	connm := map[[4]int]connection{}
	epm := map[string]map[Pid][]int{}

	// build a map of all remote (peer) intra-host endpoints
	for pid, p := range pt {
		for i, conn := range p.Connections {
			if conn.Self == "" {
				continue
			}
			switch conn.Type {
			case "FIFO", "PIPE", "SEMA", "SHMEM", "TCP", "UDP", "UNIX", "KEVT", "KCTL":
				self := conn.Type + ": " + conn.Self
				if _, ok := epm[self]; !ok {
					epm[self] = map[Pid][]int{}
				}
				epm[self][pid] = append(epm[self][pid], i)
			}
		}
	}

	// determine all inter- and intra- connections for processes
	for pid, p := range pt {
		ppid := p.Ppid
		pp := pt[ppid]
		connm[[4]int{int(ppid), -1, int(pid), -1}] = connection{
			ftype:     "parent:" + strconv.Itoa(int(ppid)), // set for edge tooltip
			name:      "child:" + strconv.Itoa(int(pid)),
			direction: "-->>",
			self: endpoint{
				pid:     ppid,
				fd:      -1,
				command: pp.Exec,
				name:    "parent",
			},
			peer: endpoint{
				pid:     pid,
				fd:      -1,
				command: p.Exec,
				name:    "child",
			},
		}

		for _, conn := range p.Connections {
			fd := conn.Descriptor
			self := endpoint{
				pid:     pid,
				fd:      fd,
				command: p.Exec,
				name:    conn.Self,
			}

			switch conn.Type {
			case "NUL": // ignore /dev/null connection endpoints
			// case "ATALK", "FSEVENTS", "NETPOLICY", "SOCK:NDRV":
			// case "BLK", "CHR", "DIR", "KQUEUE", "LINK", "REG", "SOCK":
			case "REG":
				connm[[4]int{int(pid), int(fd), math.MaxInt32, 0}] = connection{
					ftype:     conn.Type,
					name:      conn.Name,
					direction: conn.Direction,
					self:      self,
					peer: endpoint{
						pid:  math.MaxInt32,
						name: conn.Name,
					},
				}
			case "KEVT", "KCTL":
				connm[[4]int{int(pid), int(fd), 0, 0}] = connection{
					ftype:     conn.Type,
					name:      conn.Name,
					direction: conn.Direction,
					self:      self,
					peer: endpoint{
						pid:     0,
						command: conn.Peer,
						name:    conn.Name,
					},
				}
			case "FIFO", "PIPE", "SEMA", "SHMEM", "TCP", "UDP", "UNIX":
				if conn.Peer == "" {
					continue
				}
				if _, ok := epm[conn.Type+": "+conn.Peer]; !ok {
					if conn.Type == "TCP" || conn.Type == "UDP" { // possible external connection
						host, _, _ := net.SplitHostPort(conn.Peer)
						ip := net.ParseIP(host)
						_, ok := localIps[ip.String()]
						_, ok2 := interfaces[ip.String()]
						if !(ok || ok2 || ip.IsLoopback() || ip.IsInterfaceLocalMulticast() ||
							ip.IsLinkLocalMulticast() || ip.IsLinkLocalUnicast()) {
							if _, ok := remoteHosts[ip.String()]; !ok {
								hosts, err := net.LookupAddr(ip.String())
								fmt.Fprintf(os.Stderr, "ip: %s, hosts: %v err: %v\n", ip, hosts, err)
								remoteHosts[ip.String()] = strings.Join(hosts, ", ")
							}
							connm[[4]int{-1, -1, int(pid), int(fd)}] = connection{
								ftype: conn.Type,
								name:  conn.Name,
								self: endpoint{
									pid:     -1,
									fd:      -1,
									name:    conn.Peer,
									command: remoteHosts[ip.String()],
								},
								peer: self,
							}
						}
					}
					continue
				}

				key := conn.Type + ": " + conn.Peer
				rpids := make([]Pid, len(epm[key]))
				i := 0
				for rpid := range epm[key] {
					rpids[i] = rpid
					i++
				}
				sort.Slice(rpids, func(i, j int) bool {
					return rpids[i] < rpids[j]
				})

				for _, rpid := range rpids {
					if pid == rpid { // ignore intra-process connections
						continue
					}
					ix := epm[key][rpid]
					rp := pt[rpid]
					for _, i := range ix {
						rconn := rp.Connections[i]
						if !(conn.Type == rconn.Type &&
							conn.Peer == rconn.Self &&
							(rconn.Peer == conn.Self ||
								rconn.Peer == "" && (conn.Type == "SHMEM" || conn.Type == "UNIX"))) {
							continue
						}

						// ignore connection if previously identified
						rfd := rconn.Descriptor
						if _, ok := connm[[4]int{int(pid), int(fd), int(rpid), int(rfd)}]; ok {
							continue
						}
						if _, ok := connm[[4]int{int(rpid), int(rfd), int(pid), int(fd)}]; ok {
							continue
						}

						direction := conn.Direction
						s := self
						r := endpoint{
							pid:     rpid,
							fd:      rfd,
							command: rp.Exec,
							name:    conn.Peer,
						}
						if len(pt[pid].ancestors) > len(pt[rpid].ancestors) {
							if direction == "-->>" {
								direction = "<<--"
							} else if direction == "<<--" {
								direction = "-->>"
							}
							s, r = r, s
						}
						connm[[4]int{int(s.pid), int(s.fd), int(r.pid), int(r.fd)}] = connection{
							ftype:     conn.Type,
							name:      conn.Name,
							direction: direction,
							self:      s,
							peer:      r,
						}
					}
				}
			}
		}
	}

	keys := make([][4]int, len(connm))
	i := 0
	for key := range connm {
		keys[i] = key
		i++
	}
	sort.Slice(keys, func(i, j int) bool {
		var di, dj int
		if keys[i][0] == -1 { // remote host connection
			di = -1
		} else {
			di = len(pt[Pid(keys[i][0])].ancestors)
		}
		if keys[j][0] == -1 { // remote host connection
			dj = -1
		} else {
			dj = len(pt[Pid(keys[j][0])].ancestors)
		}
		return di < dj ||
			di == dj && (keys[i][0] < keys[j][0] ||
				keys[i][0] == keys[j][0] && (keys[i][2] < keys[j][2] ||
					keys[i][2] == keys[j][2] && (keys[i][1] < keys[j][1] ||
						keys[i][1] == keys[j][1] && keys[i][3] < keys[j][3])))
	})

	conns := make([]connection, len(keys))
	for i, key := range keys {
		conns[i] = connm[key]
	}

	return conns
}
