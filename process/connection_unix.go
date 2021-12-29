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
	"sync"

	"github.com/zosmac/gomon/core"
	"github.com/zosmac/gomon/log"
)

var (
	// hnMap caches resolver host name lookup.
	hnMap  = map[string]string{}
	hnLock sync.RWMutex

	// regex for parsing lsof output lines from lsof command.
	regex = regexp.MustCompile(
		`^(?:(?P<header>COMMAND.*)|====(?P<trailer>\d\d:\d\d:\d\d)====.*|` +
			`(?P<command>[^ ]+)[ ]+` +
			`(?P<pid>[^ ]+)[ ]+` +
			`(?:[^ ]+)[ ]+` + // USER
			`(?:(?P<fd>\d+)|fp\.|mem|cwd|rtd)` +
			`(?P<mode> |[rwu-][rwuNRWU]?)[ ]+` +
			`(?P<type>(?:[^ ]+|))[ ]+` +
			`(?P<device>(?:0x[0-9a-f]+|\d+,\d+|kpipe|upipe|))[ ]+` +
			`(?:[^ ]+|)[ ]+` + // SIZE/OFF
			`(?P<node>(?:\d+|TCP|UDP|))[ ]+` +
			`(?P<name>.*))$`,
	)

	// rgxgroups maps names of capture groups to indices.
	rgxgroups = func() map[captureGroup]int {
		g := map[captureGroup]int{}
		for _, name := range regex.SubexpNames() {
			g[captureGroup(name)] = regex.SubexpIndex(name)
		}
		return g
	}()
)

const (
	// lsof line regular expressions named capture groups.
	groupHeader  captureGroup = "header"
	groupTrailer captureGroup = "trailer"
	groupCommand captureGroup = "command"
	groupPid     captureGroup = "pid"
	groupFd      captureGroup = "fd"
	groupMode    captureGroup = "mode"
	groupType    captureGroup = "type"
	groupDevice  captureGroup = "device"
	groupNode    captureGroup = "node"
	groupName    captureGroup = "name"
)

type (
	// captureGroup is the name of a reqular expression capture group.
	captureGroup string
)

// hostname resolves the host name for an ip address.
func hostname(addr string) string {
	ip, port, _ := net.SplitHostPort(addr)
	hnLock.Lock()
	defer hnLock.Unlock()
	host, ok := hnMap[ip]
	if ok {
		if host == "" {
			host = ip
		}
	} else {
		hnMap[ip] = ""
		host = ip
		go func() {
			if hosts, err := net.LookupAddr(ip); err == nil {
				host = hosts[0]
				if i, ok := interfaces[ip]; ok {
					interfaces[host] = i
				}
			} else {
				host = ip
			}
			hnLock.Lock()
			hnMap[ip] = host
			hnLock.Unlock()
		}()
	}
	return net.JoinHostPort(host, port)
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

	core.Register(func() {
		cmd.Process.Kill()
		cmd.Wait()
	})

	core.LogInfo(fmt.Errorf("start [%d] %q", cmd.Process.Pid, cmd.String()))

	go parseOutput(stdout)

	return nil
}

// parseOutput reads the stdout of the command.
func parseOutput(stdout io.ReadCloser) {
	epm := map[Pid]Connections{}

	sc := bufio.NewScanner(stdout)
	for sc.Scan() {
		match := regex.FindStringSubmatch(sc.Text())
		if len(match) == 0 || match[0] == "" {
			continue
		}
		if header := match[rgxgroups[groupHeader]]; header != "" {
			continue
		}
		if trailer := match[rgxgroups[groupTrailer]]; trailer != "" {
			epLock.Lock()
			epMap = epm
			epm = map[Pid]Connections{}
			epLock.Unlock()
			continue
		}

		command := match[rgxgroups[groupCommand]]
		pid, _ := strconv.Atoi(match[rgxgroups[groupPid]])
		fd, _ := strconv.Atoi(match[rgxgroups[groupFd]])
		mode := match[rgxgroups[groupMode]][0]
		fdType := match[rgxgroups[groupType]]
		device := match[rgxgroups[groupDevice]]
		node := match[rgxgroups[groupNode]]
		name := match[rgxgroups[groupName]]

		var self, peer string

		switch fdType {
		case "REG":
			if runtime.GOOS == "linux" && name != "" && pid != os.Getpid() {
				log.Watch(name, pid)
			}
		case "BLK", "DIR", "LINK",
			"CHAN", "FSEVENT", "KQUEUE", "NEXUS", "NPOLICY", "PSXSHM",
			"ndrv", "unknown":
		case "CHR":
			if name == os.DevNull {
				fdType = "NUL"
			}
		case "FIFO":
			if mode == 'w' {
				peer = name
			} else {
				self = name
			}
		case "PIPE", "unix":
			peer = name
			if len(peer) > 2 && peer[:2] == "->" {
				peer = peer[2:] // strip "->"
			}
			name = device
			self = device
		case "IPv4", "IPv6":
			var state string
			fdType = node
			split := strings.Split(name, " ")
			if len(split) > 1 {
				state = split[1]
			}
			split = strings.Split(split[0], "->")
			self = hostname(split[0])
			if len(split) > 1 {
				peer = hostname(strings.Split(split[1], " ")[0])
			} else {
				self += " " + state
			}
		case "systm":
			self = device
		case "key":
			name = device
			self = device
		case "PSXSEM":
			self = device
			peer = device
		}

		ep := Connection{
			Descriptor: fd,
			Type:       fdType,
			Name:       name,
			Direction:  accmode(mode),
			Self:       self,
			Peer:       peer,
		}

		core.LogDebug(fmt.Errorf("endpoint %s:%s: %q[%d:%d] %s->%s", fdType, name, command, pid, fd, self, peer))

		epm[Pid(pid)] = append(epm[Pid(pid)], ep)
	}
}

// accmode determines the I/O direction.
func accmode(mode byte) string {
	switch mode {
	case 'r':
		return "<<--"
	case 'w':
		return "-->>"
	case 'u':
		return "<-->"
	}
	return ""
}
