// Copyright © 2021 The Gomon Project.

package system

import (
	"time"

	"github.com/zosmac/gomon/message"
	"github.com/zosmac/gomon/process"
)

func init() {
	message.Document(&measurement{})
}

type (
	// Id identifies the message.
	Id struct {
	}

	// Properties defines measurement properties.
	Properties struct {
		Uname    string    `json:"uname" gomon:"property"`
		Boottime time.Time `json:"boottime" gomon:"property"`
	}

	// LoadAverage captured by loadAverage.
	LoadAverage struct {
		OneMinute     float64 `json:"one_minute" gomon:"gauge,none"`
		FiveMinute    float64 `json:"five_minute" gomon:"gauge,none"`
		FifteenMinute float64 `json:"fifteen_minute" gomon:"gauge,none"`
	}

	// CPU holds the CPU metrics for the system and for an individual processor.
	CPU struct {
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
		CPU             CPU               `json:"cpu" gomon:""`
		CPUCount        int               `json:"cpu_count" gomon:"gauge,count"`
		Cpus            []CPU             `json:"cpus" gomon:""`
		Memory          Memory            `json:"memory" gomon:""`
		Swap            Swap              `json:"swap" gomon:""`
		Processes       process.ProcStats `json:"processes" gomon:""`
	}

	// measurement for the message.
	measurement struct {
		message.Header `gomon:""`
		Id             `json:"id" gomon:""`
		Properties     `gomon:""`
		Metrics        `gomon:""`
	}
)

// Events returns the list of acceptable Event values for this message.
func (m *measurement) Events() []string {
	return message.MeasureEvents.Values()
}

// ID returns the identifier for the system message.
func (*measurement) ID() string {
	return ""
}
