// Copyright © 2021-2023 The Gomon Project.

package system

import (
	"time"

	"github.com/zosmac/gomon/message"
	"github.com/zosmac/gomon/process"
)

func init() {
	message.Define(&Measurement{})
}

type (
	// EventID identifies the message.
	EventID struct {
		Name string `json:"name" gomon:"property"`
	}

	// Properties defines measurement properties.
	Properties struct {
		Boottime time.Time `json:"boottime" gomon:"property"`
	}

	// LoadAverage captured by loadAverage.
	LoadAverage struct {
		OneMinute     float64 `json:"one_minute" gomon:"gauge,none"`
		FiveMinute    float64 `json:"five_minute" gomon:"gauge,none"`
		FifteenMinute float64 `json:"fifteen_minute" gomon:"gauge,none"`
	}

	// Cpu holds the Cpu metrics for the system and for an individual processor.
	Cpu struct {
		Total   time.Duration `json:"total" gomon:"counter,ns"`
		User    time.Duration `json:"user" gomon:"counter,ns"`
		System  time.Duration `json:"system" gomon:"counter,ns"`
		Idle    time.Duration `json:"idle" gomon:"counter,ns"`
		Nice    time.Duration `json:"nice,omitempty" gomon:"counter,ns,linux"`
		IoWait  time.Duration `json:"io_wait,omitempty" gomon:"counter,ns,linux"`
		Stolen  time.Duration `json:"stolen,omitempty" gomon:"counter,ns,linux"`
		Irq     time.Duration `json:"irq,omitempty" gomon:"counter,ns,linux"`
		SoftIrq time.Duration `json:"soft_irq,omitempty" gomon:"counter,ns,linux"`
	}

	// Memory contains the system's memory metrics.
	Memory struct {
		Total      int `json:"total" gomon:"gauge,B"`
		Free       int `json:"free" gomon:"gauge,B"`
		Used       int `json:"used" gomon:"gauge,B"`
		FreeActual int `json:"free_actual" gomon:"gauge,B"`
		UsedActual int `json:"used_actual" gomon:"gauge,B"`
	}

	// Swap contains the system's swap metrics.
	Swap struct {
		Total int `json:"total" gomon:"gauge,B"`
		Free  int `json:"free" gomon:"gauge,B"`
		Used  int `json:"used" gomon:"gauge,B"`
	}

	// Metrics defines measurement metrics.
	Metrics struct {
		Uptime          time.Duration `json:"uptime" gomon:"counter,ns"`
		Rlimits         `gomon:""`
		LoadAverage     LoadAverage       `json:"load_average" gomon:""`
		ContextSwitches int               `json:"context_switches,omitempty" gomon:"counter,count,!darwin"`
		Cpu             Cpu               `json:"cpu" gomon:""`
		CpuCount        int               `json:"cpu_count" gomon:"gauge,count"`
		Cpus            []Cpu             `json:"cpus" gomon:""`
		Memory          Memory            `json:"memory" gomon:""`
		Swap            Swap              `json:"swap" gomon:""`
		ProcessStats    process.ProcStats `json:"process_stats" gomon:""`
	}

	// Measurement defines the properties and metrics of a system measurement.
	Measurement struct {
		message.Header[message.MeasureEvent] `gomon:""`
		EventID                              `json:"event_id" gomon:""`
		Properties                           `gomon:""`
		Metrics                              `gomon:""`
	}
)

// Events returns the list of acceptable Event values for this message.
func (*Measurement) Events() []string {
	return message.MeasureEvents.ValidValues()
}

// ID returns the identifier for the system message.
func (m *Measurement) ID() string {
	return m.EventID.Name
}
