// Copyright © 2021 The Gomon Project.

package process

import (
	"time"

	"github.com/zosmac/gomon/message"
)

func init() {
	message.Document(&measurement{})
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
		Exec string   `json:"exec" gomon:"property"`
		Args []string `json:"args" gomon:"property"`
		Envs []string `json:"envs" gomon:"property"`
	}

	// Directories reports the process' root and current working directories.
	Directories struct {
		Cwd  string `json:"cwd" gomon:"property"`
		Root string `json:"root" gomon:"property"`
	}

	// Props defines measurement properties.
	Props struct {
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

	// Connection represents a process connection to a data source.
	Connection struct {
		Descriptor int    `json:"descriptor" gomon:"property"`
		Type       string `json:"type" gomon:"property"`
		Name       string `json:"name" gomon:"property"`
		Direction  string `json:"direction" gomon:"property"`
		Self       string `json:"self" gomon:"property"`
		Peer       string `json:"peer" gomon:"property"`
	}

	// Connections records all the process' data connections.
	Connections []Connection

	// measurement for the message.
	measurement struct {
		ancestors      []Pid
		message.Header `gomon:""`
		Id             id `json:"id" gomon:""`
		Props          `gomon:""`
		Metrics        `gomon:""`
		Connections    `json:"connections" gomon:""`
	}
)

// String returns the source value of the message as a string.
func (so processSource) String() string {
	return string(so)
}

// ValidValues returns the valid source values for the message.
func (processSource) ValidValues() message.ValidValues {
	return processSources
}

// Sources returns the list of acceptable Source values for this message.
func (*measurement) Sources() []string {
	return processSources.Values()
}

// Events returns the list of acceptable Event values for this message.
func (*measurement) Events() []string {
	return message.MeasureEvents.Values()
}

// ID returns the identifier for a process message.
func (m *measurement) ID() string {
	return m.Id.Name + "[" + m.Id.Pid.String() + "]"
}
