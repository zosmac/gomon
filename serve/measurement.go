// Copyright © 2021-2023 The Gomon Project.

package serve

import (
	"time"

	"github.com/zosmac/gomon/message"
)

func init() {
	message.Define(&Measurement{})
}

type (
	// EventID identifies the message.
	EventID struct {
		Name string `json:"name" gomon:"property"`
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
		HttpRequests int `json:"http_requests" gomon:"counter,count"`
		Prometheus   `gomon:""`
		LokiStreams  int `json:"loki_streams" gomon:"counter,count"`
	}

	// Measurement defines the properties and metrics of a gomon server measurement.
	Measurement struct {
		message.Header[message.MeasureEvent] `gomon:""`
		EventID                              `json:"event_id" gomon:""`
		Properties                           `gomon:""`
		Metrics                              `gomon:""`
	}
)

var (
	// measures records the metrics for the server's operations.
	measures = Measurement{
		Header:  message.Measurement(),
		EventID: EventID{Name: "gomon"},
	}
)

// Events returns the list of acceptable Event values for this message.
func (*Measurement) Events() []string {
	return message.MeasureEvents.ValidValues()
}

// ID returns the identifier for a server message.
func (m *Measurement) ID() string {
	return m.EventID.Name
}
