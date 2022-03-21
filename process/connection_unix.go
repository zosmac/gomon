// Copyright © 2021 The Gomon Project.

//go:build !windows
// +build !windows

package process

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/zosmac/gomon/core"
	"github.com/zosmac/gomon/logs"
)

var (
	// lsofRegex for parsing lsof output lines from lsof command.
	lsofRegex = regexp.MustCompile(
		`^(?P<command>[^ ]+)[ ]+` +
			`(?P<pid>\d+)[ ]+` +
			`(?:\d+)[ ]+` + // USER
			`(?:(?P<fd>\d+)|fp\.|mem|cwd|rtd)` +
			`(?P<mode> |[rwu-][rwuNRWU]?)[ ]+` +
			`(?P<type>(?:[^ ]+|))[ ]+` +
			`(?P<device>(?:0x[0-9a-f]+|\d+,\d+|kpipe|upipe|))[ ]+` +
			`(?:[^ ]+|)[ ]+` + // SIZE/OFF
			`(?P<node>(?:[^ ]+|))`,
	)

	// lsofGroups maps capture group names to indices.
	lsofGroups = func() map[string]int {
		g := map[string]int{}
		for _, name := range lsofRegex.SubexpNames() {
			g[name] = lsofRegex.SubexpIndex(name)
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
)

const (
	// lsof output lines regular expression capture group names.
	groupCommand = "command"
	groupPid     = "pid"
	groupFd      = "fd"
	groupMode    = "mode"
	groupType    = "type"
	groupDevice  = "device"
	groupNode    = "node"
)

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

// lsofCommand starts the lsof command to capture process connections.
func lsofCommand() error {
	cmd := hostCommand() // perform OS specific customizations for command
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return core.Error("stdout pipe failed", err)
	}
	cmd.Stderr = nil // sets to /dev/null
	if err := cmd.Start(); err != nil {
		return core.Error("start failed", err)
	}

	core.LogInfo(fmt.Errorf("start [%d] %q", cmd.Process.Pid, cmd.String()))

	core.Register(func() {
		cmd.Process.Kill()
		cmd.Wait()
	})

	go parseLsof(stdout)

	return nil
}

// parseLsof parses each line of stdout from the command.
func parseLsof(stdout io.ReadCloser) {
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			buf = buf[:n]
			core.LogError(fmt.Errorf("parseLsof() panicked, %v\n%s", r, buf))
		}
	}()

	epm := map[Pid][]Connection{}
	var nameIndex int
	sc := bufio.NewScanner(stdout)
	for sc.Scan() {
		text := sc.Text()
		if strings.HasPrefix(text, "COMMAND") {
			nameIndex = strings.Index(text, "NAME")
			continue
		} else if strings.HasPrefix(text, "====") {
			epLock.Lock()
			epMap = epm
			epm = map[Pid][]Connection{}
			epLock.Unlock()
			continue
		}
		match := lsofRegex.FindStringSubmatch(text[:nameIndex])
		if len(match) == 0 || match[0] == "" {
			continue
		}

		// command := match[lsofGroups[groupCommand]]
		pid, _ := strconv.Atoi(match[lsofGroups[groupPid]])
		// fd, _ := strconv.Atoi(match[lsofGroups[groupFd]])
		mode := match[lsofGroups[groupMode]][0]
		fdType := match[lsofGroups[groupType]]
		device := match[lsofGroups[groupDevice]]
		node := match[lsofGroups[groupNode]]
		peer := text[nameIndex:]

		var self string

		switch fdType {
		case "REG":
			if runtime.GOOS == "linux" && peer != "" && pid != os.Getpid() {
				logs.Watch(peer, pid)
			}
		case "BLK", "CHR", "DIR", "LINK", "PSXSHM", "KQUEUE":
		case "FSEVENT", "NEXUS", "NPOLICY", "ndrv", "systm", "unknown":
		case "CHAN":
			fdType = device
		case "key", "PSXSEM":
			peer = device
		case "FIFO":
			if mode != 'w' {
				self = peer
				peer = ""
			}
		case "PIPE", "unix":
			self = device
			if len(peer) > 2 && peer[:2] == "->" {
				peer = peer[2:] // strip "->"
			}
		case "IPv4", "IPv6":
			fdType = node
			split := strings.Split(peer, " ")
			split = strings.Split(split[0], "->")
			if len(split) > 1 {
				self = addZone(split[0])
				peer = addZone(split[1])
			} else {
				self = device
				peer = addZone((split[0]))
			}
		}

		if self == "" && peer == "" {
			peer = fdType // treat like data connection
		}

		if peer != os.DevNull {
			epm[Pid(pid)] = append(epm[Pid(pid)],
				Connection{
					Type:      fdType,
					Direction: accmode(mode),
					Self:      Endpoint{Name: self, Pid: Pid(pid)},
					Peer:      Endpoint{Name: peer},
				},
			)
		}
		if fdType == "unix" && peer[:2] != "0x" {
			epm[Pid(pid)] = append(epm[Pid(pid)],
				Connection{
					Type:      fdType,
					Direction: accmode(mode),
					Self:      Endpoint{Pid: Pid(pid)},
					Peer:      Endpoint{Name: peer},
				},
			)
		}
	}
}

// accmode determines the I/O direction.
func accmode(mode byte) string {
	switch mode {
	case 'r':
		return "<<--"
	case 'w':
		return "-->>"
	}
	return "<-->"
}
