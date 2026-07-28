// Copyright © 2021-2023 The Gomon Project.

package message

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"time"

	"github.com/zosmac/gocore"
)

type (
	// field records the attributes of a field for documenting.
	field struct {
		key      string
		Name     string
		Property bool   // true if field is a property
		Type     string // metric type
		Unit     string // metric unit
		reflect.Value
	}

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

	// fields contains a definition for each message's fields.
	fields []field

	// Messages contains a map of all message definitions.
	Messages = map[string][]field{}

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
	s, _, _ := strings.Cut(filepath.Base(f.Function), ".")
	return s
}

// Define a Message's Content.
func Define(m Content) {
	fs := gocore.Format("", "", 0, reflect.ValueOf(m),
		func(name, tag string, val reflect.Value) any {
			return messageField(m, name, tag, val)
		},
	)
	src := filepath.Base(reflect.ValueOf(m).Elem().Type().PkgPath())
	k := src + " |" + strings.Join(m.Events(), "|")
	Messages[k] = make([]field, len(fs))
	for i, f := range fs {
		Messages[k][i] = f.(field)
		fields = append(fields, f.(field))
	}
}

// messageField interprets a gomon tag for each message field.
func messageField(m Content, name, tag string, val reflect.Value) field {
	if max.Name < len(name) {
		max.Name = len(name)
	}

	s := strings.Split(tag, ",")
	t := ""
	u := ""
	if len(s) > 0 {
		t = s[0]
	}
	if len(s) > 1 {
		u = s[1]
	}

	key := filepath.Base(reflect.ValueOf(m).Elem().Type().PkgPath()) + " |" + strings.Join(m.Events(), "|")

	switch t {
	case "":
		return field{
			key:   key,
			Name:  name,
			Value: val,
		}
	case "property":
		return field{
			key:      key,
			Name:     name,
			Property: true,
			Value:    val,
		}
	}

	if max.Type < len(t) {
		max.Type = len(t)
	}
	if max.Unit < len(u) {
		max.Unit = len(u)
	}

	return field{
		key:   key,
		Name:  name,
		Type:  t,
		Unit:  u,
		Value: val,
	}
}
