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
	logEvent string

	// Id identifies the message.
	Id struct {
		Name   string `json:"name" gomon:"property"`
		Pid    int    `json:"pid" gomon:"property"`
		Sender string `json:"sender" gomon:"property"`
	}

	// message defines the properties of a log message.
	observation struct {
		message.Header[logEvent] `gomon:"property"`
		Id                       `json:"id" gomon:""`
		Message                  string `json:"message" gomon:"property"`
	}
)

const (
	// message events.
	eventTrace logEvent = "trace"
	eventDebug logEvent = "debug"
	eventInfo  logEvent = "info"
	eventWarn  logEvent = "warn"
	eventError logEvent = "error"
	eventFatal logEvent = "fatal"
)

var (
	// logLevels valid event values for messages, in severity order.
	logEvents = gocore.ValidValue[logEvent]{}.Define(
		eventTrace,
		eventDebug,
		eventInfo,
		eventWarn,
		eventError,
		eventFatal,
	)
)

// Events returns the list of acceptable Event values for this message.
func (*observation) Events() []string {
	return logEvents.ValidValues()
}

// ID returns the identifier for a log message message.
func (obs *observation) ID() string {
	return obs.Id.Name + "[" + strconv.Itoa(obs.Id.Pid) + "] (" + obs.Id.Sender + ")"
}
