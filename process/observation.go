// Copyright © 2021 The Gomon Project.

package process

import (
	"github.com/zosmac/gomon/core"
	"github.com/zosmac/gomon/message"
)

func init() {
	message.Document(&observation{})
}

type (
	// processEvent type.
	processEvent string

	// message defines the properties of a process message.
	observation struct {
		message.Header[processEvent] `gomon:""`
		Id                           `json:"id" gomon:""`
		Message                      string `json:"message" gomon:"property"`
	}
)

const (
	// message events.
	processFork   processEvent = "fork"
	processExec   processEvent = "exec"
	processExit   processEvent = "exit"
	processSetuid processEvent = "setuid" // linux only
	processSetgid processEvent = "setgid" // linux only
)

var (
	// processEvents valid event values for messages.
	processEvents = core.ValidValue[processEvent]{}.Define(
		processFork,
		processExec,
		processExit,
		processSetuid,
		processSetgid,
	)
)

// Events returns the list of acceptable Event values for this message.
func (*observation) Events() []string {
	return processEvents.ValidValues()
}

// ID returns the identifier for a process message message.
func (obs *observation) ID() string {
	return obs.Id.Name + "[" + obs.Id.Pid.String() + "]"
}
