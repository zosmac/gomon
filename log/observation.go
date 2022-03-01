// Copyright © 2021 The Gomon Project.

package log

import (
	"strconv"

	"github.com/zosmac/gomon/message"
)

func init() {
	message.Document(&observation{})
}

const (
	// message sources.
	sourceLog    logSource = "log"
	sourceOSLog  logSource = "oslog"
	sourceSyslog logSource = "syslog"

	// message events.
	levelFatal logLevel = "fatal"
	levelError logLevel = "error"
	levelWarn  logLevel = "warn"
	levelInfo  logLevel = "info"
	levelDebug logLevel = "debug"
	levelTrace logLevel = "trace"
)

var (
	// logSources valid source values for messages.
	logSources = message.ValidValues{
		sourceLog,
		sourceOSLog,
		sourceSyslog,
	}

	// logLevels valid event values for messages, in severity order.
	logLevels = message.ValidValues{
		levelTrace,
		levelDebug,
		levelInfo,
		levelWarn,
		levelError,
		levelFatal,
	}
)

type (
	// logSource type.
	logSource string

	// logLevel type.
	logLevel string

	// Id identifies the message.
	Id struct {
		Name   string `json:"name" gomon:"property"`
		Pid    int    `json:"pid" gomon:"property"`
		Sender string `json:"sender" gomon:"property"`
	}

	// message defines the properties of a log message.
	observation struct {
		message.Header `gomon:"property"`
		Id             `json:"id" gomon:""`
		Message        string `json:"message" gomon:"property"`
	}
)

// String returns the source value of the message as a string.
func (so logSource) String() string {
	return string(so)
}

// ValidValues returns the valid source values for the message.
func (logSource) ValidValues() message.ValidValues {
	return logSources
}

// String returns the source value of the message as a string.
// This method is already defined in flag.go.
// func (ev logLevel) String() string {
// 	return string(ev)
// }

// ValidValues returns the valid event values for the message.
func (logLevel) ValidValues() message.ValidValues {
	return logLevels
}

// Sources returns the list of acceptable Source values for this message.
func (*observation) Sources() []string {
	return logSources.Values()
}

// Events returns the list of acceptable Event values for this message.
func (*observation) Events() []string {
	return logLevels.Values()
}

// ID returns the identifier for a log message message.
func (obs *observation) ID() string {
	return obs.Id.Name + "[" + strconv.Itoa(obs.Id.Pid) + "] (" + obs.Id.Sender + ")"
}
