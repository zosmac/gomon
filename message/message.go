// Copyright © 2021 The Gomon Project.

package message

import (
	"fmt"
	"time"

	"github.com/zosmac/gomon/core"
)

type (
	ValidValues []ValidValue

	// ValidValue interface defines valid values for an event type.
	ValidValue interface {
		fmt.Stringer
		ValidValues() ValidValues
	}

	// Header for a message.
	Header struct {
		Timestamp time.Time  `json:"timestamp" gomon:"property"`
		Host      string     `json:"host" gomon:"property"`
		Source    ValidValue `json:"source" gomon:"property"`
		Event     ValidValue `json:"event" gomon:"property"`
	}

	// Content interface methods for all messages.
	Content interface {
		Sources() []string
		Events() []string
		ID() string
	}
)

// Values returns an ordered list of valid values for the type.
func (vs ValidValues) Values() []string {
	ss := make([]string, len(vs))
	for i, v := range vs {
		ss[i] = v.String()
	}
	return ss
}

// IsValid returns whether a value is valid.
func (vs ValidValues) IsValid(s string) bool {
	for _, v := range vs {
		if s == v.String() {
			return true
		}
	}
	return false
}

// Index returns the position of a value in the valid value list.
func (vs ValidValues) Index(vv ValidValue) int {
	for i, v := range vs {
		if vv == v {
			return i
		}
	}
	return -1
}

// Observation initializes message header for observation.
func Observation(t time.Time, source ValidValue, event ValidValue) Header {
	return Header{
		Timestamp: t,
		Host:      core.Hostname,
		Source:    source,
		Event:     event,
	}
}

type measureEvent string

const (
	measure measureEvent = "measure"
)

var (
	MeasureEvents = ValidValues{
		measure,
	}
)

// String returns the source value of the message as a string.
func (ev measureEvent) String() string {
	return string(ev)
}

func (measureEvent) ValidValues() ValidValues {
	return MeasureEvents
}

// Measurement initializes message header for Measurement.
func Measurement(source ValidValue) Header {
	return Header{
		Timestamp: time.Now(),
		Host:      core.Hostname,
		Source:    source,
		Event:     measure,
	}
}
