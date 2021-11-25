// Copyright © 2021 The Gomon Project.

//go:build !windows
// +build !windows

package process

/*
#include <libproc.h>
*/
import "C"

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/zosmac/gomon/core"
)

var (
	// regex for parsing lsof output lines from lsof command.
	regex = regexp.MustCompile(
		`^(?:(?P<header>COMMAND.*)|====(?P<trailer>\d\d:\d\d:\d\d)====.*|` +
			`(?P<command>[^ ]+)[ ]+` +
			`(?P<pid>[^ ]+)[ ]+` +
			`(?:[^ ]+)[ ]+` + // USER
			`(?P<fd>(?:\d+|fp\.))` +
			`(?P<mode>(?: |[-rwu](?:.?)))[ ]+` + // ignore possible lock character after mode
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

// lsofCommand starts the lsof command to capture process connections
func lsofCommand(ready chan<- struct{}) {
	cmd := hostCommand() // perform OS specific customizations for command

	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			buf = buf[:n]
			core.LogError(fmt.Errorf("command panicked %q[%d]\n%v\n%s", cmd.String(), cmd.Process.Pid, r, buf))
		}
	}()

	core.LogInfo(fmt.Errorf("fork command to capture open process descriptors: %q", cmd.String()))

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		core.LogError(fmt.Errorf("pipe to stdout failed %v", err))
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		core.LogError(fmt.Errorf("pipe to stderr failed %v", err))
		return
	}

	if err := cmd.Start(); err != nil {
		core.LogError(fmt.Errorf("command failed %q[%d] %v", cmd.String(), cmd.Process.Pid, err))
		return
	}

	ready <- struct{}{}

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
		mode := match[rgxgroups[groupMode]]
		fdType := match[rgxgroups[groupType]]
		device := match[rgxgroups[groupDevice]]
		node := match[rgxgroups[groupNode]]
		name := match[rgxgroups[groupName]]

		var self, peer string

		switch fdType {
		case "BLK", "DIR", "REG", "LINK",
			"CHAN", "FSEVENT", "KQUEUE", "NEXUS", "NPOLICY", "PSXSHM":
		case "CHR":
			if name == os.DevNull {
				fdType = "NUL"
			}
		case "FIFO":
			if mode == "w" {
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
				state = split[0]
			}
			split = strings.Split(split[0], "->")
			self = split[0]
			if len(split) == 2 {
				peer = strings.Split(split[1], " ")[0]
			} else {
				self += " " + state
			}
			name = device
		case "systm":
			self = device
			peer = "kernel"
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

	core.LogError(fmt.Errorf("scanning output failed %q[%d] %v", cmd.String(), cmd.Process.Pid, sc.Err()))

	if buf, err := io.ReadAll(stderr); err != nil || len(buf) > 0 {
		core.LogError(fmt.Errorf("command error log %q[%d] %v\n%s", cmd.String(), cmd.Process.Pid, err, buf))
	}

	err = cmd.Wait()
	code := cmd.ProcessState.ExitCode()
	core.LogError(fmt.Errorf("command failed %q[%d] %d %v", cmd.String(), cmd.Process.Pid, code, err))

	os.Exit(code)
}

// accmode determines the I/O direction.
func accmode(mode string) string {
	switch mode {
	case "r":
		return "<<--"
	case "w":
		return "-->>"
	case "u":
		return "<-->"
	}
	return ""
}
