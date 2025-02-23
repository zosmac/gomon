// Copyright © 2021-2023 The Gomon Project.

//go:build !windows

package process

import (
	"bufio"
	"context"
	"fmt"
	"math"
	"net"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/zosmac/gocore"
	"github.com/zosmac/gomon/logs"
)

var (
	// endpoints of processes periodically populated by lsof.
	epMap  = map[Pid][]Connection{}
	epLock sync.RWMutex

	// headerRegex for parsing lsof header line of lsof command.
	headerRegex = regexp.MustCompile(
		`^(?P<command>COMMAND) ` +
			`(?P<pid>[ ]*PID) ` +
			`(?P<user>[ ]*USER) ` +
			`(?P<fd>[ ]*FD)` +
			`(?P<mode> )` +
			`(?P<lock> ) ` +
			`(?P<type>[ ]*TYPE) ` +
			`(?P<device>[ ]*DEVICE) ` +
			`(?P<sizeoff>[ ]*SIZE/OFF) ` +
			`(?P<node>[ ]*NODE) ` +
			`(?P<name>[ ]*NAME)[ ]*$`,
	)

	// headerGroups maps capture group names to indices.
	headerGroups = func() map[string]int {
		g := map[string]int{}
		for _, name := range headerRegex.SubexpNames() {
			g[name] = headerRegex.SubexpIndex(name)
		}
		return g
	}()

	// nameRegex for parsing the lsof NAME field of Linux 'unix' type files.
	nameRegex = regexp.MustCompile(
		`^(?:(?P<name>[^ ]*) |)type=[A-Z]* (?:->INO=` +
			`(?P<inode>\d*) ` +
			`(?P<pid>\d*)|).*\(` +
			`(?P<state>[A-Z]*)\)$`,
	)

	// nameGroups maps capture group names to indices.
	nameGroups = func() map[string]int {
		g := map[string]int{}
		for _, name := range nameRegex.SubexpNames() {
			g[name] = nameRegex.SubexpIndex(name)
		}
		return g
	}()

	// zoneregex determines if a link local address embeds a zone index.
	zoneregex = regexp.MustCompile(`^((fe|FE)80):(\d{1,2})(::.*)$`)

	// zones maps local ip addresses to their network zones.
	zones = func() map[string]string {
		zm := map[string]string{}
		if nis, err := net.Interfaces(); err == nil {
			for _, ni := range nis {
				zm[strconv.FormatUint(uint64(ni.Index), 16)] = ni.Name
				if addrs, err := ni.Addrs(); err == nil {
					for _, addr := range addrs {
						if ip, _, err := net.ParseCIDR(addr.String()); err == nil {
							zm[ip.String()] = ni.Name
						}
					}
				}
				if addrs, err := ni.MulticastAddrs(); err == nil {
					for _, addr := range addrs {
						if ip, _, err := net.ParseCIDR(addr.String()); err == nil {
							zm[ip.String()] = ni.Name
						}
					}
				}
			}
		}
		return zm
	}()

	// hostPid is a pseudo pid for a network node (e.g. remote host).
	hostPid Pid = -1

	// dataPid is a pseudo pid for a data node (e.g. file, pipe, unix socket, kernel connection).
	dataPid Pid = math.MaxInt32

	// nodes maps a node's name to its pseudo pid.
	nodes = map[string]Pid{}
)

const (
	// lsof header line regular expression capture group names.
	groupCommand = "command"
	groupPid     = "pid"
	groupUser    = "user"
	groupFd      = "fd"
	groupMode    = "mode"
	groupLock    = "lock"
	groupType    = "type"
	groupDevice  = "device"
	groupSizeOff = "sizeoff"
	groupNode    = "node"
	groupName    = "name"

	// lsof NAME field regular expression capture group names for 'unix' type files.
	groupInode = "inode"
	groupState = "state"
)

func getEndpoints() map[Pid][]Connection {
	epLock.RLock()
	defer epLock.RUnlock()
	return epMap
}

func setEndpoints(epm map[Pid][]Connection) {
	epLock.Lock()
	defer epLock.Unlock()
	epMap = epm
}

// endpoints starts the lsof command to capture process connection endpoints.
func endpoints(ctx context.Context) error {
	stdout, err := gocore.Spawn(ctx, lsofCommand())
	if err != nil {
		return gocore.Error("Spawn", err, map[string]string{
			"command": "lsof",
		})
	}

	go parseLsof(stdout)

	return nil
}

// parseLsof parses each line of stdout from the command.
func parseLsof(sc *bufio.Scanner) {
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			buf = buf[:n]
			gocore.Error("parseLsof", fmt.Errorf("%v", r), map[string]string{
				"stacktrace": string(buf),
			}).Err()
		}
	}()

	epm := map[Pid][]Connection{}
	var indexUser, indexFd, indexMode /* indexLock, */, indexType, indexDevice, indexSize, indexNode, indexName int

	for sc.Scan() {
		text := sc.Text()
		if strings.HasPrefix(text, "COMMAND") {
			// lsof header: COMMAND PID USER FDml TYPE DEVICE SIZE/OFF NODE NAME
			indices := headerRegex.FindStringSubmatchIndex(text)
			if indices == nil {
				os.Exit(13)
			}
			indexUser = indices[headerGroups[groupUser]*2]
			indexFd = indices[headerGroups[groupFd]*2]
			indexMode = indices[headerGroups[groupMode]*2]
			// indexLock = indices[headerGroups[groupLock]*2]
			indexType = indices[headerGroups[groupType]*2]
			indexDevice = indices[headerGroups[groupDevice]*2]
			indexSize = indices[headerGroups[groupSizeOff]*2]
			indexNode = indices[headerGroups[groupNode]*2]
			indexName = indices[headerGroups[groupName]*2]
			continue
		} else if strings.HasPrefix(text, "====") {
			connections(epm)
			setEndpoints(epm)
			epm = map[Pid][]Connection{}
			continue
		}

		fd := strings.TrimSpace(text[indexFd:indexMode])
		if _, err := strconv.Atoi(fd); err != nil {
			continue
		}

		// command := strings.Fields(text[:indexUser])[0]           // COMMAND and PID fields may be jammed together
		pid, _ := strconv.Atoi(strings.Fields(text[:indexUser])[1]) // so read as one field and split
		// user := strings.TrimSpace(text[indexUser:indexFd])
		mode := text[indexMode]
		// lock := text[indexLock]
		fdType := strings.TrimSpace(text[indexType:indexDevice])
		device := strings.TrimSpace(text[indexDevice:indexSize])
		// size := strings.TrimSpace(text[indexSize:indexNode])
		node := strings.TrimSpace(text[indexNode:indexName])
		name := text[indexName:]

		var self, peer string
		var peerPid Pid
		var ok bool

		switch fdType {
		case "CHAN":
			fdType += ":" + device
			fallthrough
		case "REG", "BLK", "CHR", "DIR", "LINK", "PSXSHM", "KQUEUE",
			"FSEVENT", "NEXUS", "NPOLICY", "ndrv", "systm", "unknown",
			"netlink", "a_inode":
			if fdType == "REG" {
				if runtime.GOOS == "linux" && name != "" && pid != os.Getpid() {
					logs.Watch(name, pid)
				}
			}
			peer = name
			if peerPid, ok = nodes[peer]; !ok {
				peerPid = dataPid
				nodes[peer] = dataPid
				dataPid += 1
			}
		case "key", "PSXSEM":
			peer = device
			if peerPid, ok = nodes[peer]; !ok {
				peerPid = dataPid
				nodes[peer] = dataPid
				dataPid += 1
			}
		case "FIFO":
			switch runtime.GOOS {
			case "darwin": // FIFO is only for named pipes
				if mode != 'w' {
					self = name
					peer = node
				} else {
					self = node
					peer = name
				}
			case "linux": // FIFO can be named or unnamed pipe
				fields := strings.Fields(name)
				self = node
				peer = fields[0]
				if len(fields) > 1 {
					pid, _ := strconv.Atoi(strings.Split(fields[1], ",")[0])
					peerPid = Pid(pid)
				} else {
					continue // no connection
				}
			}
		case "PIPE": // darwin distinguishes unnamed pipe from FIFO
			if len(name) < 2 || name[:2] != "->" {
				continue // no connection
			}
			self = device
			peer = name[2:] // strip "->"
		case "unix":
			switch runtime.GOOS {
			case "darwin":
				self = device
				if len(name) > 2 && name[:2] == "->" {
					peer = name[2:] // strip "->"
				} else {
					peer = name // unix socket file
				}
			case "linux":
				self = node
				matches := nameRegex.FindStringSubmatch(name)
				if peer = matches[nameGroups[groupInode]]; len(peer) == 0 {
					peer = matches[nameGroups[groupName]]
				}
				pid, _ := strconv.Atoi(matches[nameGroups[groupPid]])
				peerPid = Pid(pid)
			}
			if peer == "" {
				continue // no connection
			}
		case "IPv4", "IPv6":
			fdType = node
			split := strings.Split(name, " ")
			split = strings.Split(split[0], "->")
			if len(split) > 1 {
				self = addZone(split[0])
				peer = addZone(split[1])
			} else { // listen
				self = device
				peer = addZone(split[0])
			}
			if _, _, err := net.SplitHostPort(peer); err == nil { // host connection
				var ok bool
				if peerPid, ok = nodes[node+peer]; !ok {
					peerPid = hostPid
					nodes[node+peer] = hostPid
					hostPid -= 1
				}
			}
		}

		if self == "" && peer == "" {
			peer = fdType // treat like data connection
		}

		if name != os.DevNull {
			epm[Pid(pid)] = append(epm[Pid(pid)],
				Connection{
					Type: fdType,
					Self: Endpoint{Name: self, Pid: Pid(pid)},
					Peer: Endpoint{Name: peer, Pid: peerPid},
				},
			)
		}
		if fdType == "unix" && peer[0] == '/' { // add unix socket file also as a data connection
			if peerPid, ok = nodes[peer]; !ok {
				peerPid = dataPid
				nodes[peer] = dataPid
				dataPid += 1
			}
			epm[Pid(pid)] = append(epm[Pid(pid)],
				Connection{
					Type: fdType,
					Self: Endpoint{Pid: Pid(pid)},
					Peer: Endpoint{Name: peer, Pid: peerPid},
				},
			)
		}
	}
}

func addZone(addr string) string {
	ip, port, _ := net.SplitHostPort(addr)
	match := zoneregex.FindStringSubmatch(ip)
	if match != nil { // strip the zone index from the ipv6 link local address
		ip = match[1] + match[4]
		if zone, ok := zones[match[3]]; ok {
			ip += "%" + zone
		}
	} else if zone, ok := zones[ip]; ok {
		ip += "%" + zone
	}
	return net.JoinHostPort(ip, port)
}
