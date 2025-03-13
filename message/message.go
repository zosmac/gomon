// Copyright © 2021-2023 The Gomon Project.

package message

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/zosmac/gocore"
)

type (
	// MeasureEvent defines the event type for a measurement.
	MeasureEvent string

	// Header for a message.
	Header[T ~string] struct {
		Timestamp time.Time `json:"timestamp" gomon:"property"`
		Host      string    `json:"host" gomon:"property"`
		Platform  string    `json:"platform" gomon:"property"`
		Source    string    `json:"source" gomon:"property"`
		Event     T         `json:"event" gomon:"property"`
	}

	// Content interface methods for all messages.
	Content interface {
		Events() []string
		ID() string
	}
)

const (
	// measure is the event for all measurements.
	measure MeasureEvent = "measure"
)

var (
	// host identifies the local host.
	host, _ = os.Hostname()

	// platform identifies the local OS.
	platform = runtime.GOOS + "_" + runtime.GOARCH

	// MeasureEvents has only the single type "measure".
	MeasureEvents = gocore.ValidValue[MeasureEvent]{}.Define(measure)
)

// Measurement initializes the message header for measurement.
// Measurement types are distinguised by their source.
func Measurement() Header[MeasureEvent] {
	return Header[MeasureEvent]{
		Timestamp: time.Now(),
		Host:      host,
		Platform:  platform,
		Source:    source(),
		Event:     measure,
	}
}

// Observation initializes the message header for an observation.
// An observer (source) may detect several types of events, so the
// source qualifies an event type by its origin.
func Observation[T ~string](t time.Time, event T) Header[T] {
	return Header[T]{
		Timestamp: t,
		Host:      host,
		Platform:  platform,
		Source:    source(),
		Event:     event,
	}
}

// source qualifies the event type of an observation/measurement.
func source() string {
	pc := []uintptr{0}
	runtime.Callers(3, pc)
	fs := runtime.CallersFrames(pc)
	f, _ := fs.Next()
	return strings.Split(filepath.Base(f.Function), ".")[0]
}
