// Copyright © 2021 The Gomon Project.

package filesystem

import (
	"github.com/zosmac/gomon/message"
)

func init() {
	message.Document(&measurement{})
}

const (
	// measurement sources.
	sourceFilesystem filesystemSource = "filesystem"
)

var (
	// filesystemSources valid source values for messages.
	filesystemSources = message.ValidValues{
		sourceFilesystem,
	}
)

type (
	// filesystemSource type.
	filesystemSource string

	// Id identifies the message.
	Id struct {
		Mount string `json:"mount" gomon:"property"`
		Path  string `json:"path" gomon:"property"`
	}

	// Properties defines measurement properties.
	Properties struct {
		Type      string `json:"type" gomon:"property"`
		Options   string `json:"options,omitempty" gomon:"property,,linux"`
		DriveType string `json:"drive_type,omitempty" gomon:"property,,windows"`
		Device    string `json:"device,omitempty" gomon:"property,,windows"`
	}

	// Metrics defines measurement metrics.
	Metrics struct {
		Total     int `json:"total" gomon:"gauge,B"`
		Used      int `json:"used" gomon:"gauge,B"`
		Free      int `json:"free" gomon:"gauge,B"`
		Available int `json:"available" gomon:"gauge,B"`
		Files     int `json:"files" gomon:"gauge,count"`
		FreeFiles int `json:"free_files" gomon:"gauge,count"`
	}

	// measurement for the message.
	measurement struct {
		message.Header `gomon:""`
		Id             `json:"id" gomon:""`
		Properties     `gomon:""`
		Metrics        `gomon:""`
	}
)

// String returns the source value of the message as a string.
func (so filesystemSource) String() string {
	return string(so)
}

// ValidValues returns the valid source values for the message.
func (filesystemSource) ValidValues() message.ValidValues {
	return filesystemSources
}

// Sources returns the list of acceptable Source values for this message.
func (*measurement) Sources() []string {
	return filesystemSources.Values()
}

// Events returns the list of acceptable Event values for this message.
func (*measurement) Events() []string {
	return message.MeasureEvents.Values()
}

// ID returns the identifier for a filesystem message.
func (m *measurement) ID() string {
	return m.Id.Mount + ":" + m.Id.Path
}
