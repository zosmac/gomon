// Copyright © 2021-2023 The Gomon Project.

package process

import (
	"github.com/zosmac/gocore"
	"github.com/zosmac/gomon/message"
)

func init() {
	message.Define(&Observation{})
}

type (
	// processEvent type.
	processEvent string

	// Observation defines the properties of a process message.
	Observation struct {
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
func (*Observation) Events() []string {
	return processEvents.ValidValues()
}

// ID returns the identifier for a process message message.
func (obs *Observation) ID() string {
	return obs.EventID.Name + "[" + obs.EventID.Pid.String() + "]"
}
