// Copyright © 2021-2023 The Gomon Project.

package file

import (
	"github.com/zosmac/gocore"
	"github.com/zosmac/gomon/message"
)

func init() {
	message.Define(&observation{})
}

type (
	// fileEvent type.
	fileEvent string

	// Id identifies the message.
	Id struct {
		Name    string `json:"name" gomon:"property"`
		EventID string `json:"event_id,omitempty" gomon:"property"`
	}

	// message defines the properties of a file update message.
	observation struct {
		message.Header[fileEvent] `gomon:""`
		Id                        `json:"id" gomon:""`
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
func (*observation) Events() []string {
	return fileEvents.ValidValues()
}

// ID returns the identifier for a file update message message.
func (obs *observation) ID() string {
	return obs.Id.Name
}
