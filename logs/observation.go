// Copyright © 2021-2023 The Gomon Project.

package logs

import (
	"strconv"

	"github.com/zosmac/gocore"
	"github.com/zosmac/gomon/message"
)

func init() {
	message.Define(&observation{})
}

type (
	// LogEvent log level event type.
	LogEvent string

	// Id identifies the message.
	Id struct {
		Name   string `json:"name" gomon:"property"`
		Pid    int    `json:"pid" gomon:"property"`
		Sender string `json:"sender" gomon:"property"`
	}

	// message defines the properties of a log message.
	observation struct {
		message.Header[LogEvent] `gomon:"property"`
		Id                       `json:"id" gomon:""`
		Message                  string `json:"message" gomon:"property"`
	}
)

const (
	// message events.
	LevelTrace LogEvent = "trace"
	LevelDebug LogEvent = "debug"
	LevelInfo  LogEvent = "info"
	LevelWarn  LogEvent = "warn"
	LevelError LogEvent = "error"
	LevelFatal LogEvent = "fatal"
)

var (
	// logLevels valid event values for messages, in severity order.
	logEvents = gocore.ValidValue[LogEvent]{}.Define(
		LevelTrace,
		LevelDebug,
		LevelInfo,
		LevelWarn,
		LevelError,
		LevelFatal,
	)

	EventMap = map[LogEvent]gocore.LogLevel{
		LevelTrace: gocore.LevelTrace,
		LevelDebug: gocore.LevelDebug,
		LevelInfo:  gocore.LevelInfo,
		LevelWarn:  gocore.LevelWarn,
		LevelError: gocore.LevelError,
		LevelFatal: gocore.LevelFatal,
	}
)

// Events returns the list of acceptable Event values for this message.
func (*observation) Events() []string {
	return logEvents.ValidValues()
}

// ID returns the identifier for a log message message.
func (obs *observation) ID() string {
	return obs.Id.Name + "[" + strconv.Itoa(obs.Id.Pid) + "] (" + obs.Id.Sender + ")"
}
