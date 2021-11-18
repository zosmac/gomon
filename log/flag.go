// Copyright © 2021 The Gomon Project.

package log

import (
	"errors"
	"regexp"
	"runtime"
	"strings"

	"github.com/zosmac/gomon/core"
)

var (
	// flags defines the command line flags.
	flags = struct {
		logLevel
		// following flags are for linux only
		logDirectory    string
		logRegex        core.Regexp
		logRegexExclude core.Regexp
	}{
		logLevel:     levelError,
		logDirectory: "/var/log",
		logRegex: core.Regexp{
			Regexp: regexp.MustCompile(`^.*\.log$`),
		},
		logRegexExclude: core.Regexp{
			Regexp: regexp.MustCompile(`^$`),
		},
	}
)

// init initializes the command line flags.
func init() {
	s := strings.Join(logLevels.Values(), "|")
	core.Flags.Var(&flags.logLevel, "loglevel", "[-loglevel "+s+"]",
		"Filter out log entries below this logging level threshold `"+s+"`")

	if runtime.GOOS == "linux" {
		core.Flags.Var(&flags.logDirectory, "logdirectory", "[-logdirectory <path>]",
			"The `path` to the top of a directory hierarchy of log files to tail with names matching -logregex")
		core.Flags.Var(&flags.logRegex, "logregex", "[-logregex <expression>]",
			"A regular `expression` for selecting log files from the directory hierarchy to watch")
		core.Flags.Var(&flags.logRegexExclude, "logregexexclude", "[-logregexexclude <expression>]",
			"A regular `expression` for excluding log files from the directory hierarchy to watch")
	}
}

// Set is a flag.Value interface method to enable logLevel as a command line flag.
func (l *logLevel) Set(level string) error {
	level = strings.ToLower(level)
	if l.ValidValues().IsValid(level) {
		*l = logLevel(level)
		return nil
	}
	return errors.New("valid values are " + strings.Join(l.ValidValues().Values(), ", "))
}

// String is a flag.Value interface method to enable logLevel as a command line flag.
func (l logLevel) String() string {
	return string(l)
}
