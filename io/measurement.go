// Copyright © 2021-2023 The Gomon Project.

package io

import (
	"time"

	"github.com/zosmac/gomon/message"
)

func init() {
	message.Define(&measurement{})
}

type (
	// EventID identifies the message.
	EventID struct {
		Device string `json:"device" gomon:"property"`
	}

	// Properties defines measurement properties.
	Properties struct {
		Major          string `json:"major,omitempty" gomon:"property,,!windows"`
		Minor          string `json:"minor,omitempty" gomon:"property,,!windows"`
		Drive          string `json:"drive,omitempty" gomon:"property,,windows"`
		DriveType      string `json:"drive_type,omitempty" gomon:"property,,windows"`
		Path           string `json:"path,omitempty" gomon:"property,,windows"`
		FilesystemType string `json:"filesystem_type,omitempty" gomon:"property,,windows"`
		TotalSize      int    `json:"total_size" gomon:"property"`
		BlockSize      int    `json:"block_size" gomon:"property"`
	}

	// Metrics defines measurement metrics.
	Metrics struct {
		ReadOperations  int           `json:"read_operations" gomon:"counter,count"`
		Read            int           `json:"read,omitempty" gomon:"counter,B,!linux"`
		ReadTime        time.Duration `json:"read_time" gomon:"counter,ns"`
		WriteOperations int           `json:"write_operations" gomon:"counter,count"`
		Write           int           `json:"write,omitempty" gomon:"counter,B,!linux"`
		WriteTime       time.Duration `json:"write_time" gomon:"counter,ns"`
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

// ID returns the identifier for an I/O message.
func (m *measurement) ID() string {
	return m.EventID.Device
}
