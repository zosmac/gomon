// Copyright © 2021-2023 The Gomon Project.

package logs

import (
	"fmt"
	"regexp"
	"runtime"
	"strings"

	"github.com/zosmac/gocore"
)

var (
	// flags defines the command line flags.
	Flags = struct {
		LogEvent
		// following flags are for linux only
		logDirectory    string
		logRegex        gocore.Regexp
		logRegexExclude gocore.Regexp
	}{
		LogEvent:     LevelError,
		logDirectory: "/var/log",
		logRegex: gocore.Regexp{
			Regexp: regexp.MustCompile(`^.*\.log$`),
		},
		logRegexExclude: gocore.Regexp{
			Regexp: regexp.MustCompile(`^$`),
		},
	}
)

// init initializes the command line flags.
func init() {
	s := strings.Join(logEvents.ValidValues(), "|")
	gocore.Flags.Var(
		&Flags.LogEvent,
		"loglevel",
		"[-loglevel "+s+"]",
		"Filter out log entries below this logging level threshold `"+s+"`",
	)

	if runtime.GOOS == "linux" {
		gocore.Flags.Var(
			&Flags.logDirectory,
			"logdirectory",
			"[-logdirectory <path>]",
			"The `path` to the top of a directory hierarchy of log files to tail with names matching -logregex",
		)
		gocore.Flags.Var(
			&Flags.logRegex,
			"logregex",
			"[-logregex <expression>]",
			"A regular `expression` for selecting log files from the directory hierarchy to watch",
		)
		gocore.Flags.Var(
			&Flags.logRegexExclude,
			"logregexexclude",
			"[-logregexexclude <expression>]",
			"A regular `expression` for excluding log files from the directory hierarchy to watch",
		)
	}
}

// Set is a flag.Value interface method to enable logLevel as a command line flag.
func (l *LogEvent) Set(level string) error {
	level = strings.ToLower(level)
	if logEvents.IsValid(LogEvent(level)) {
		*l = LogEvent(level)
		return nil
	}
	return fmt.Errorf("valid values are %s", strings.Join(logEvents.ValidValues(), ", "))
}

// String is a flag.Value interface method to enable logLevel as a command line flag.
func (l *LogEvent) String() string {
	return string(*l)
}
