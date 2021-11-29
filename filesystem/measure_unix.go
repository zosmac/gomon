// Copyright © 2021 The Gomon Project.

//go:build !windows

package filesystem

import (
	"fmt"
	"syscall"

	"github.com/zosmac/gomon/core"
)

// metrics captures a filesystem's metrics.
func metrics(path string) Metrics {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		core.LogError(fmt.Errorf("statfs %v", err))
		return Metrics{}
	}

	m := Metrics{
		Total:     int(stat.Blocks) * int(stat.Bsize),
		Free:      int(stat.Bfree) * int(stat.Bsize),
		Available: int(stat.Bavail) * int(stat.Bsize),
		Files:     int(stat.Files),
		FreeFiles: int(stat.Ffree),
	}
	m.Used = m.Total - m.Free

	return m
}
