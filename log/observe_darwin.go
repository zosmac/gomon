// Copyright © 2021 The Gomon Project.

package log

/*
#include <libproc.h>
*/
import "C"

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/zosmac/gomon/core"
	"github.com/zosmac/gomon/message"
)

const (
	// log record regular expressions named capture groups.
	groupTimestamp captureGroup = "timestamp"
	groupLevel     captureGroup = "level"
	groupHost      captureGroup = "host"
	groupProcess   captureGroup = "process"
	groupPid       captureGroup = "pid"
	groupThread    captureGroup = "thread"
	groupSender    captureGroup = "sender"
	groupSubCat    captureGroup = "subcat"
	groupMessage   captureGroup = "message"
)

var (
	// osLogLevels maps gomon log levels to OSLog message types
	osLogLevels = map[logLevel]int{
		levelTrace: 0,  // Default
		levelDebug: 0,  // Default
		levelInfo:  1,  // Info
		levelWarn:  2,  // Debug
		levelError: 16, // Error
		levelFatal: 17, // Fault
	}

	// syslogLevels maps gomon log levels to syslog log levels
	syslogLevels = map[logLevel]string{
		levelTrace: "7", // Debug
		levelDebug: "7", // Debug
		levelInfo:  "6", // Info, Notice
		levelWarn:  "4", // Warning
		levelError: "3", // Error, Critical
		levelFatal: "1", // Alert, Emergency
	}
)

// open obtains a watch handle for observer.
func open() error {
	return nil
}

// listen uses the macOS syslog and log commands to stream log entries.
func listen() {
	go syslogCommand() // use macOS syslog command to stream syslog entries
	go logCommand()    // use macOS log command to stream OSLogStore entries
}

// logCommand starts the log command to capture OSLog entries (using OSLogStore API directly is MUCH slower)
func logCommand() {
	// regex for parsing log entries from macOS log command
	regex := regexp.MustCompile(
		`^(?P<timestamp>\d\d\d\d-\d\d-\d\d \d\d:\d\d:\d\d\.\d\d\d\d\d\d[+-]\d\d\d\d) ` +
			`(?P<thread>[^ ]+)[ ]+` +
			`(?P<level>[^ ]+)[ ]+` +
			`(?P<activity>[^ ]+)[ ]+` +
			`(?P<pid>\d+)[ ]+` +
			`(?P<ttl>\d+)[ ]+` +
			`(?P<process>[^:]+): ` +
			`\((?P<sender>[^\)]+)\) ` +
			`(?:\[(?P<subcat>[^\]]+)\] |)` +
			`(?P<message>.*)$`)

	// groups maps names of capture groups to indices
	groups := func() map[captureGroup]int {
		g := map[captureGroup]int{}
		for _, name := range regex.SubexpNames() {
			g[captureGroup(name)] = regex.SubexpIndex(name)
		}
		return g
	}()

	predicate := fmt.Sprintf(
		"(eventType == 'logEvent') AND (messageType >= %d) AND (NOT eventMessage BEGINSWITH[cd] '%s')",
		osLogLevels[flags.logLevel],
		"System Policy: gomon",
	)
	cmdline := append(strings.Fields("log stream --predicate"), predicate)
	if err := parseLog(cmdline, regex, groups, "2006-01-02 15:04:05Z0700", sourceOSLog); err != nil {
		core.LogError(err)
	}
}

// syslogCommand starts the syslog command to capture syslog entries
func syslogCommand() {
	// regex for parsing output from the syslog -w -T utc.3 command
	regex := regexp.MustCompile(
		`^(?P<timestamp>\d\d\d\d-\d\d-\d\d \d\d:\d\d:\d\d\.\d\d\dZ) ` +
			`(?P<host>[^ ]+) ` +
			`(?P<process>[^\[]+)\[` +
			`(?P<pid>\d+)\] ` +
			`(?:\((?P<sender>(?:\[[\d]+\]|)[^\)]+|)\) |)` +
			`<(?P<level>[A-Z][a-z]*)>: ` +
			`(?P<message>.*)$`)

	// groups maps names of capture groups to indices
	groups := func() map[captureGroup]int {
		g := map[captureGroup]int{}
		for _, name := range regex.SubexpNames() {
			g[captureGroup(name)] = regex.SubexpIndex(name)
		}
		return g
	}()

	cmdline := append(strings.Fields("syslog -w 0 -T utc.3 -k Level Nle"), syslogLevels[flags.logLevel])
	if err := parseLog(cmdline, regex, groups, "2006-01-02 15:04:05Z", sourceSyslog); err != nil {
		core.LogError(err)
	}
}

func parseLog(cmdline []string, regex *regexp.Regexp, groups map[captureGroup]int, format string, source logSource) (err error) {
	cmd := exec.Command(cmdline[0], cmdline[1:]...)
	// ensure that no open descriptors propagate to child
	if n := C.proc_pidinfo(
		C.int(os.Getpid()),
		C.PROC_PIDLISTFDS,
		0,
		nil,
		0,
	); n >= 3*C.PROC_PIDLISTFD_SIZE {
		cmd.ExtraFiles = make([]*os.File, (n/C.PROC_PIDLISTFD_SIZE)-3) // close gomon files in child
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return core.NewError("stdout pipe failed", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return core.NewError("stderr pipe failed", err)
	}

	if err := cmd.Start(); err != nil {
		return core.NewError(filepath.Base(cmd.Path)+" command failed", err)
	}
	core.Register(func() {
		cmd.Process.Kill()
		if buf, err := io.ReadAll(stderr); err != nil || len(buf) > 0 {
			core.LogInfo(fmt.Errorf("%s[%d] %v stderr:\n%s", filepath.Base(cmd.Path), cmd.Process.Pid, err, buf))
		}
		err := cmd.Wait()
		core.LogInfo(fmt.Errorf("%s[%d] %d %v", filepath.Base(cmd.Path), cmd.Process.Pid, cmd.ProcessState.ExitCode(), err))
	})

	sc := bufio.NewScanner(stdout)
	if source == sourceOSLog {
		sc.Scan()                           // read first line from log command
		core.LogInfo(fmt.Errorf(sc.Text())) //  and echo it as startup message
		sc.Scan()                           // this line is column headers
		sc.Text()                           //  so just ignore it
	}

	for sc.Scan() {
		match := regex.FindStringSubmatch(sc.Text())
		if len(match) == 0 || match[0] == "" {
			continue
		}

		t, _ := time.Parse(format, match[groups[groupTimestamp]])
		pid, _ := strconv.Atoi(match[groups[groupPid]])

		sender := match[groups[groupSender]]
		if cg, ok := groups[groupSubCat]; ok {
			sender = match[cg] + ":" + sender
		}

		messageChan <- &observation{
			Header: message.Observation(t, source, levelMap[strings.ToLower(match[groups[groupLevel]])]),
			Id: id{
				Name:   match[groups[groupProcess]],
				Pid:    pid,
				Sender: sender,
			},
			Message: match[groups[groupMessage]],
		}
	}

	return sc.Err()
}

// Remove exited processes' logs from observation, which is a noop for Darwin.
func Remove(pid []int) {
}
