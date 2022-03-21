// Copyright © 2021 The Gomon Project.

package message

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
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
		Source    string     `json:"source" gomon:"property"`
		Event     ValidValue `json:"event" gomon:"property"`
	}

	// Content interface methods for all messages.
	Content interface {
		Events() []string
		ID() string
	}
)

func source() string {
	pc := []uintptr{0}
	runtime.Callers(3, pc)
	fs := runtime.CallersFrames(pc)
	f, _ := fs.Next()
	return strings.Split(filepath.Base(f.Function), ".")[0]
}

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
func Observation(t time.Time, event ValidValue) Header {
	return Header{
		Timestamp: t,
		Host:      core.Hostname,
		Source:    source(),
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

// String returns the event value of the message as a string.
func (ev measureEvent) String() string {
	return string(ev)
}

// ValidValues returns the valid event values for the message.
func (measureEvent) ValidValues() ValidValues {
	return MeasureEvents
}

// Measurement initializes message header for Measurement.
func Measurement() Header {
	return Header{
		Timestamp: time.Now(),
		Host:      core.Hostname,
		Source:    source(),
		Event:     measure,
	}
}
