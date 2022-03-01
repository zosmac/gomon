// Copyright © 2021 The Gomon Project.

package process

import "github.com/zosmac/gomon/message"

func init() {
	message.Document(&observation{})
}

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
	processEvents = message.ValidValues{
		processFork,
		processExec,
		processExit,
		processSetuid,
		processSetgid,
	}
)

type (
	// processEvent type.
	processEvent string

	// message defines the properties of a process message.
	observation struct {
		message.Header `gomon:""`
		Id             `json:"id" gomon:""`
		Message        string `json:"message" gomon:"property"`
	}
)

// String returns the source value of the message as a string.
func (ev processEvent) String() string {
	return string(ev)
}

// ValidValues returns the valid event values for the message.
func (processEvent) ValidValues() message.ValidValues {
	return processEvents
}

// Sources returns the list of acceptable Source values for this message.
func (*observation) Sources() []string {
	return processSources.Values()
}

// Events returns the list of acceptable Event values for this message.
func (*observation) Events() []string {
	return processEvents.Values()
}

// ID returns the identifier for a process message message.
func (obs *observation) ID() string {
	return obs.Id.Name + "[" + obs.Id.Pid.String() + "]"
}
