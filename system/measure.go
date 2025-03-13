// Copyright © 2021-2023 The Gomon Project.

package system

import (
	"runtime"

	"github.com/zosmac/gocore"
	"github.com/zosmac/gomon/message"
	"github.com/zosmac/gomon/process"
)

// Measure captures the system's metrics.
func Measure(ps process.ProcStats) message.Content {
	header := message.Measurement()
	mem, swap := memory()
	return &measurement{
		Header: header,
		Id: Id{
			Name: uname,
		},
		Properties: Properties{
			Boottime: gocore.Boottime,
		},
		Metrics: Metrics{
			Uptime:          header.Timestamp.Sub(gocore.Boottime),
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
