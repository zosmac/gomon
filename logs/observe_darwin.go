// Copyright © 2021-2023 The Gomon Project.

package logs

/*
#include <libproc.h>
*/
import "C"

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/zosmac/gocore"
)

var (
	ctx, cncl = context.WithCancel(context.Background())

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

	// logRegex for parsing output from the log stream --predicate command.
	logRegex = regexp.MustCompile(
		`^(?P<timestamp>\d\d\d\d-\d\d-\d\d \d\d:\d\d:\d\d\.\d\d\d\d\d\d[+-]\d\d\d\d) ` +
			`(?P<thread>[^ ]+)[ ]+` +
			`(?P<level>[^ ]+)[ ]+` +
			`(?P<activity>[^ ]+)[ ]+` +
			`(?P<pid>\d+)[ ]+` +
			`(?P<ttl>\d+)[ ]+` +
			`(?P<process>[^:]+): ` +
			`\((?P<sender>[^\)]+)\) ` +
			`(?:\[(?P<subcat>[^\]]+)\] |)` +
			`(?P<message>.*)$`,
	)

	// syslogRegex for parsing output from the syslog -w -T utc.3 command.
	syslogRegex = regexp.MustCompile(
		`^(?P<timestamp>\d\d\d\d-\d\d-\d\d \d\d:\d\d:\d\d\.\d\d\dZ) ` +
			`(?P<host>[^ ]+) ` +
			`(?P<process>[^\[]+)\[` +
			`(?P<pid>\d+)\] ` +
			`(?:\((?P<sender>(?:\[[\d]+\]|)[^\)]+|)\) |)` +
			`<(?P<level>[A-Z][a-z]*)>: ` +
			`(?P<message>.*)$`,
	)
)

// open obtains a watch handle for observer.
func open() error {
	return nil
}

// close OS resources.
func close() {
	cncl()
}

// observe starts the macOS log and syslog commands as sub-processes to stream log entries.
func observe() error {
	err := logCommand(ctx)
	if err == nil {
		err = syslogCommand(ctx)
	}
	return err
}

// logCommand starts the log command to capture OSLog entries (using OSLogStore API directly is MUCH slower)
func logCommand(ctx context.Context) error {
	predicate := fmt.Sprintf(
		"(eventType == 'logEvent') AND (messageType >= %d) AND (NOT eventMessage BEGINSWITH[cd] '%s')",
		osLogLevels[flags.logLevel],
		"System Policy: gomon",
	)

	sc, err := gocore.Spawn(ctx, append(strings.Fields("log stream --predicate"), predicate))
	if err != nil {
		return gocore.Error("Spawn(log stream)", err)
	}

	sc.Scan() // ignore first output line from log command
	sc.Text() //  (it just echoes the filter)
	sc.Scan() // ignore second output line
	sc.Text() //  (it is column headers)

	go parseLog(sc, logRegex, "2006-01-02 15:04:05Z0700")

	return nil
}

// syslogCommand starts the syslog command to capture syslog entries
func syslogCommand(ctx context.Context) error {
	sc, err := gocore.Spawn(ctx, append(strings.Fields("syslog -w 0 -T utc.3 -k Level Nle"),
		syslogLevels[flags.logLevel]),
	)
	if err != nil {
		return gocore.Error("Spawn(syslog)", err)
	}

	go parseLog(sc, syslogRegex, "2006-01-02 15:04:05Z")

	return nil
}

// Watch adds a process' logs to watch to the observer, which is a noop for Darwin.
func Watch(name string, pid int) {
}

// Remove exited processes' logs from observation, which is a noop for Darwin.
func Remove(pid []int) {
}
