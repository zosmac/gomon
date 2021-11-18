// Copyright © 2021 The Gomon Project.

package file

import (
	"github.com/zosmac/gomon/message"
)

func init() {
	message.Document(&observation{})
}

const (
	// message sources.
	sourceFile fileSource = "file"

	// message events.
	fileCreate fileEvent = "create"
	fileRename fileEvent = "rename"
	fileUpdate fileEvent = "update"
	fileDelete fileEvent = "delete"
)

var (
	// fileSources valid source values for messages.
	fileSources = message.ValidValues{
		sourceFile,
	}

	// fileEvents valid event values for messages.
	fileEvents = message.ValidValues{
		fileCreate,
		fileRename,
		fileUpdate,
		fileDelete,
	}
)

type (
	// fileSource type.
	fileSource string

	// fileEvent type.
	fileEvent string

	// id identifies the message.
	id struct {
		Name string `json:"name" gomon:"property"`
	}

	// message defines the properties of a file update message.
	observation struct {
		message.Header `gomon:""`
		Id             id     `json:"id" gomon:""`
		Message        string `json:"message" gomon:"property"`
	}
)

// String returns the source value of the message as a string.
func (so fileSource) String() string {
	return string(so)
}

// ValidValues returns the valid source values for the message.
func (fileSource) ValidValues() message.ValidValues {
	return fileSources
}

// String returns the source value of the message as a string.
func (ev fileEvent) String() string {
	return string(ev)
}

// ValidValues returns the valid event values for the message.
func (fileEvent) ValidValues() message.ValidValues {
	return fileEvents
}

// Sources returns the list of acceptable Source values for this message.
func (*observation) Sources() []string {
	return fileSources.Values()
}

// Events returns the list of acceptable Event values for this message.
func (*observation) Events() []string {
	return fileEvents.Values()
}

// ID returns the identifier for a file update message message.
func (obs *observation) ID() string {
	return obs.Id.Name
}
