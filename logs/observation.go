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
		message.Header[logLevel] `gomon:"property"`
		Id                       `json:"id" gomon:""`
		Message                  string `json:"message" gomon:"property"`
	}
)

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
	logLevels = gocore.ValidValue[logLevel]{}.Define(
		levelTrace,
		levelDebug,
		levelInfo,
		levelWarn,
		levelError,
		levelFatal,
	)
)

// Events returns the list of acceptable Event values for this message.
func (*observation) Events() []string {
	return logLevels.ValidValues()
}

// ID returns the identifier for a log message message.
func (obs *observation) ID() string {
	return obs.Id.Name + "[" + strconv.Itoa(obs.Id.Pid) + "] (" + obs.Id.Sender + ")"
}
