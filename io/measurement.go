// Copyright © 2021 The Gomon Project.

package io

import (
	"time"

	"github.com/zosmac/gomon/message"
)

func init() {
	message.Document(&measurement{})
}

const (
	// measurement sources.
	sourceIO ioSource = "i/o"
)

var (
	// ioSources valid source values for messages.
	ioSources = message.ValidValues{
		sourceIO,
	}
)

type (
	// ioSource type.
	ioSource string

	// id identifies the message.
	id struct {
		Device string `json:"device" gomon:"property"`
	}

	// Props defines measurement properties.
	Props struct {
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
		message.Header `gomon:""`
		Id             id `json:"id" gomon:""`
		Props          `gomon:""`
		Metrics        `gomon:""`
	}
)

// String returns the source value of the message.
func (so ioSource) String() string {
	return string(so)
}

// ValidValues returns the valid source values for the message.
func (ioSource) ValidValues() message.ValidValues {
	return ioSources
}

// Sources returns the list of acceptable Source values for this message.
func (*measurement) Sources() []string {
	return ioSources.Values()
}

// Events returns the list of acceptable Event values for this message.
func (*measurement) Events() []string {
	return message.MeasureEvents.Values()
}

// ID returns the identifier for an I/O message.
func (m *measurement) ID() string {
	return m.Id.Device
}
