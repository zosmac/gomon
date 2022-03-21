// Copyright © 2021 The Gomon Project.

package logs

import (
	"strconv"

	"github.com/zosmac/gomon/message"
)

func init() {
	message.Document(&observation{})
}

const (
	// message events.
	levelFatal logLevel = "fatal"
	levelError logLevel = "error"
	levelWarn  logLevel = "warn"
	levelInfo  logLevel = "info"
	levelDebug logLevel = "debug"
	levelTrace logLevel = "trace"
)

var (
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

// String returns the event value of the message as a string.
// This method is already defined in flag.go.
// func (ev logLevel) String() string {
// 	return string(ev)
// }

// ValidValues returns the valid event values for the message.
func (logLevel) ValidValues() message.ValidValues {
	return logLevels
}

// Events returns the list of acceptable Event values for this message.
func (*observation) Events() []string {
	return logLevels.Values()
}

// ID returns the identifier for a log message message.
func (obs *observation) ID() string {
	return obs.Id.Name + "[" + strconv.Itoa(obs.Id.Pid) + "] (" + obs.Id.Sender + ")"
}
