// Copyright © 2021-2023 The Gomon Project.

package filesystem

import (
	"github.com/zosmac/gomon/message"
)

func init() {
	message.Define(&measurement{})
}

type (
	// EventID identifies the message.
	EventID struct {
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
		message.Header[message.MeasureEvent] `gomon:""`
		EventID                              `json:"event_id" gomon:""`
		Properties                           `gomon:""`
		Metrics                              `gomon:""`
	}
)

// Events returns the list of acceptable Event values for this message.
func (*measurement) Events() []string {
	return message.MeasureEvents.ValidValues()
}

// ID returns the identifier for a filesystem message.
func (m *measurement) ID() string {
	return m.EventID.Mount + ":" + m.EventID.Path
}
