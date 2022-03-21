// Copyright © 2021 The Gomon Project.

package system

import (
	"runtime"

	"github.com/zosmac/gomon/core"
	"github.com/zosmac/gomon/message"
	"github.com/zosmac/gomon/process"
)

// Measure captures the system's metrics.
func Measure(ps process.ProcStats) message.Content {
	hdr := message.Measurement()
	mem, swap := memory()
	return &measurement{
		Header: hdr,
		Properties: Properties{
			Uname:    uname(),
			Boottime: core.Boottime,
		},
		Metrics: Metrics{
			Uptime:          hdr.Timestamp.Sub(core.Boottime),
			Rlimits:         rlimits(),
			LoadAverage:     loadAverage(),
			ContextSwitches: contextSwitches(),
			CPU:             cpu(),
			CPUCount:        runtime.NumCPU(),
			Cpus:            cpus(),
			Memory:          mem,
			Swap:            swap,
			Processes:       ps,
		},
	}
}
