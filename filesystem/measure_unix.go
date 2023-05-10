// Copyright © 2021-2023 The Gomon Project.

//go:build !windows

package filesystem

import (
	"syscall"

	"github.com/zosmac/gocore"
)

// metrics captures a filesystem's metrics.
func metrics(path string) Metrics {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		gocore.Error("statfs", err).Err()
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
