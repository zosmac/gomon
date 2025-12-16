// Copyright © 2021-2023 The Gomon Project.

package process

import (
	"github.com/zosmac/gocore"
	"github.com/zosmac/gomon/message"
)

func init() {
	message.Define(&observation{})
}

type (
	// processEvent type.
	processEvent string

	// message defines the properties of a process message.
	observation struct {
		message.Header[processEvent] `gomon:""`
		EventID                      `json:"event_id" gomon:""`
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
	processEvents = gocore.ValidValue[processEvent]{}.Define(
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
	return obs.EventID.Name + "[" + obs.EventID.Pid.String() + "]"
}
