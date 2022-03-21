// Copyright © 2021 The Gomon Project.

package file

import (
	"github.com/zosmac/gomon/message"
)

func init() {
	message.Document(&observation{})
}

const (
	// message events.
	fileCreate fileEvent = "create"
	fileRename fileEvent = "rename"
	fileUpdate fileEvent = "update"
	fileDelete fileEvent = "delete"
)

var (
	// fileEvents valid event values for messages.
	fileEvents = message.ValidValues{
		fileCreate,
		fileRename,
		fileUpdate,
		fileDelete,
	}
)

type (
	// fileEvent type.
	fileEvent string

	// Id identifies the message.
	Id struct {
		Name string `json:"name" gomon:"property"`
	}

	// message defines the properties of a file update message.
	observation struct {
		message.Header `gomon:""`
		Id             `json:"id" gomon:""`
		Message        string `json:"message" gomon:"property"`
	}
)

// String returns the event value of the message as a string.
func (ev fileEvent) String() string {
	return string(ev)
}

// ValidValues returns the valid event values for the message.
func (fileEvent) ValidValues() message.ValidValues {
	return fileEvents
}

// Events returns the list of acceptable Event values for this message.
func (*observation) Events() []string {
	return fileEvents.Values()
}

// ID returns the identifier for a file update message message.
func (obs *observation) ID() string {
	return obs.Id.Name
}
