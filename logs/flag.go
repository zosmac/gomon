// Copyright © 2021-2023 The Gomon Project.

package logs

import (
	"errors"
	"regexp"
	"runtime"
	"strings"

	"github.com/zosmac/gocore"
)

var (
	// flags defines the command line flags.
	flags = struct {
		logLevel
		// following flags are for linux only
		logDirectory    string
		logRegex        gocore.Regexp
		logRegexExclude gocore.Regexp
	}{
		logLevel:     levelError,
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
	s := strings.Join(logLevels.ValidValues(), "|")
	gocore.Flags.Var(
		&flags.logLevel,
		"loglevel",
		"[-loglevel "+s+"]",
		"Filter out log entries below this logging level threshold `"+s+"`",
	)

	if runtime.GOOS == "linux" {
		gocore.Flags.Var(
			&flags.logDirectory,
			"logdirectory",
			"[-logdirectory <path>]",
			"The `path` to the top of a directory hierarchy of log files to tail with names matching -logregex",
		)
		gocore.Flags.Var(
			&flags.logRegex,
			"logregex",
			"[-logregex <expression>]",
			"A regular `expression` for selecting log files from the directory hierarchy to watch",
		)
		gocore.Flags.Var(
			&flags.logRegexExclude,
			"logregexexclude",
			"[-logregexexclude <expression>]",
			"A regular `expression` for excluding log files from the directory hierarchy to watch",
		)
	}
}

// Set is a flag.Value interface method to enable logLevel as a command line flag.
func (l *logLevel) Set(level string) error {
	level = strings.ToLower(level)
	if logLevels.IsValid(logLevel(level)) {
		*l = logLevel(level)
		return nil
	}
	return errors.New("valid values are " + strings.Join(logLevels.ValidValues(), ", "))
}

// String is a flag.Value interface method to enable logLevel as a command line flag.
func (l *logLevel) String() string {
	return string(*l)
}
