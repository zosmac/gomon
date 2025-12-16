// Copyright © 2021-2023 The Gomon Project.

package process

import (
	"time"

	"github.com/zosmac/gomon/message"
)

func init() {
	message.Define(&measurement{})
}

type (
	// ProcStats defines system level process summary metrics. Sent to system.Measure() to include in the system measure.
	ProcStats struct {
		Count  int           `json:"count" gomon:"gauge,count"`
		Active int           `json:"active" gomon:"gauge,count"`
		Execed int           `json:"execed" gomon:"gauge,count"`
		Exited int           `json:"exited" gomon:"gauge,count"`
		CPU    time.Duration `json:"cpu" gomon:"gauge,ns"`
	}

	// CommandLine contains a process' command line arguments.
	CommandLine struct {
		Executable string   `json:"executable" gomon:"property"`
		Args       []string `json:"args" gomon:"property"`
		Envs       []string `json:"envs" gomon:"property"`
	}

	// Directories reports the process' root and current working directories.
	Directories struct {
		Cwd  string `json:"cwd" gomon:"property"`
		Root string `json:"root" gomon:"property"`
	}

	// Properties defines measurement properties.
	Properties struct {
		Ppid        Pid    `json:"ppid" gomon:"property"`
		Pgid        int    `json:"pgid,omitempty" gomon:"property,,!windows"`
		Tgid        int    `json:"tgid,omitempty" gomon:"property,,linux"`
		Tty         string `json:"tty,omitempty" gomon:"property,,!windows"`
		UID         int    `json:"uid,omitempty" gomon:"property,,!windows"`
		GID         int    `json:"gid,omitempty" gomon:"property,,!windows"`
		Username    string `json:"username" gomon:"property"`
		Groupname   string `json:"groupname,omitempty" gomon:"property,,!windows"`
		Status      string `json:"status" gomon:"enum,none"`
		Nice        int    `json:"nice,omitempty" gomon:"gauge,none,!windows"`
		CommandLine `gomon:""`
		Directories `gomon:""`
	}

	// Io contains a process' I/O metrics.
	Io struct {
		ReadActual      int `json:"read_actual" gomon:"counter,B"`
		WriteActual     int `json:"write_actual" gomon:"counter,B"`
		ReadRequested   int `json:"read_requested,omitempty" gomon:"counter,B,linux"`
		WriteRequested  int `json:"write_requested,omitempty" gomon:"counter,B,!windows"`
		ReadOperations  int `json:"read_operations,omitempty" gomon:"counter,count,!darwin"`
		WriteOperations int `json:"write_operations,omitempty" gomon:"counter,count,!darwin"`
	}

	// Metrics defines measurement metrics.
	Metrics struct {
		Priority                    int           `json:"priority,omitempty" gomon:"gauge,none,!windows"`
		Threads                     int           `json:"threads" gomon:"gauge,count"`
		User                        time.Duration `json:"user" gomon:"counter,ns"`
		System                      time.Duration `json:"system" gomon:"counter,ns"`
		Total                       time.Duration `json:"total" gomon:"counter,ns"`
		Size                        int           `json:"size" gomon:"gauge,B"`
		Resident                    int           `json:"resident" gomon:"gauge,B"`
		Share                       int           `json:"share,omitempty" gomon:"gauge,B,linux"`
		VirtualMemoryMax            int           `json:"virtual_memory_max,omitempty" gomon:"counter,B,linux"`
		ResidentMemoryMax           int           `json:"resident_memory_max,omitempty" gomon:"counter,B,linux"`
		PageFaults                  int           `json:"page_faults" gomon:"counter,count"`
		MinorFaults                 int           `json:"minor_faults,omitempty" gomon:"counter,count,linux"`
		MajorFaults                 int           `json:"major_faults,omitempty" gomon:"counter,count,linux"`
		VoluntaryContextSwitches    int           `json:"voluntary_context_switches,omitempty" gomon:"counter,count,linux"`
		NonVoluntaryContextSwitches int           `json:"nonvoluntary_context_switches,omitempty" gomon:"counter,count,linux"`
		ContextSwitches             int           `json:"context_switches,omitempty" gomon:"counter,count,!windows"`
		Io                          `gomon:""`
	}

	// Endpoint identifies one end of a connection.
	Endpoint struct {
		Name string `json:"name" gomon:"property"`
		Pid  Pid    `json:"pid" gomon:"property"`
	}

	// Connection represents an inter-process or host/data connection.
	Connection struct {
		Type string   `json:"type" gomon:"property"`
		Self Endpoint `json:"self" gomon:"property"`
		Peer Endpoint `json:"peer" gomon:"property"`
	}

	// measurement for the message.
	measurement struct {
		message.Header[message.MeasureEvent] `gomon:""`
		EventID                              `json:"event_id" gomon:""`
		Properties                           `gomon:""`
		Metrics                              `gomon:""`
		Connections                          []Connection `json:"connections" gomon:""`
	}

	Process = measurement
)

// Events returns the list of acceptable Event values for this message.
func (*measurement) Events() []string {
	return message.MeasureEvents.ValidValues()
}

// ID returns the identifier for a process message.
func (m *measurement) ID() string {
	return m.EventID.Name + "[" + m.EventID.Pid.String() + "]"
}
