// Copyright © 2021-2023 The Gomon Project.

package file

import (
	"github.com/zosmac/gocore"
	"github.com/zosmac/gomon/message"
)

func init() {
	message.Define(&Observation{})
}

type (
	// fileEvent type.
	fileEvent string

	// EventID identifies the message.
	EventID struct {
		Name        string `json:"name" gomon:"property"`
		FileEventId uint64 `json:"file_event_id,omitempty" gomon:"property"`
	}

	// Observation defines the properties of a file update message.
	Observation struct {
		message.Header[fileEvent] `gomon:""`
		EventID                   `json:"event_id" gomon:""`
		Message                   string `json:"message" gomon:"property"`
	}
)

const (
	// message events.
	fileCreate fileEvent = "create"
	fileRename fileEvent = "rename"
	fileUpdate fileEvent = "update"
	fileDelete fileEvent = "delete"
)

var (
	// fileEvents valid event values for messages.
	fileEvents = gocore.ValidValue[fileEvent]{}.Define(
		fileCreate,
		fileRename,
		fileUpdate,
		fileDelete,
	)
)

// Events returns the list of acceptable Event values for this message.
func (*Observation) Events() []string {
	return fileEvents.ValidValues()
}

// ID returns the identifier for a file update message message.
func (obs *Observation) ID() string {
	return obs.EventID.Name
}
