// Copyright © 2021-2023 The Gomon Project.

package main

import (
	"time"

	"github.com/zosmac/gomon/message"
)

func init() {
	message.Define(&measurement{})
}

type (
	// Id identifies the message.
	Id struct {
	}
	// WebServer reports the address and endpoints of gomon.
	WebServer struct {
		Address   string   `json:"address" gomon:"property"`
		Endpoints []string `json:"endpoints" gomon:"property"`
	}

	// Properties defines measurement properties.
	Properties struct {
		WebServer `gomon:""`
	}

	Prometheus struct {
		Collections    int           `json:"collections" gomon:"counter,count"`
		CollectionTime time.Duration `json:"collection_time" gomon:"counter,ns"`
	}

	// Metrics defines measurement metrics.
	Metrics struct {
		HTTPRequests int `json:"http_requests" gomon:"counter,count"`
		Prometheus   `gomon:""`
		LokiStreams  int `json:"loki_streams" gomon:"counter,count"`
	}

	// measurement for the message.
	measurement struct {
		message.Header[message.MeasureEvent] `gomon:""`
		Id                                   `json:"id" gomon:""`
		Properties                           `gomon:""`
		Metrics                              `gomon:""`
	}
)

var (
	measures = measurement{
		Header: message.Measurement(),
	}
)

// Events returns the list of acceptable Event values for this message.
func (*measurement) Events() []string {
	return message.MeasureEvents.ValidValues()
}

// ID returns the identifier for an I/O message.
func (m *measurement) ID() string {
	return "gomon"
}
