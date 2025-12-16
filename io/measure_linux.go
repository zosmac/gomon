// Copyright © 2021-2023 The Gomon Project.

package io

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/zosmac/gomon/message"
)

// Measure captures system's I/O metrics.
func Measure() (ms []message.Content) {
	f, err := os.Open("/proc/diskstats")
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		strs := make([]string, 3)
		nums := make([]int, 11)
		flds := make([]any, 14)
		for i := range flds[:3] {
			flds[i] = &strs[i]
		}
		for i := range flds[3:] {
			flds[i+3] = &nums[i]
		}
		fmt.Sscanf(sc.Text(), "%s %s %s %d %d %d %d %d %d %d %d %d %d %d", flds...)

		var statfs syscall.Statfs_t
		syscall.Statfs(filepath.Join("/dev", strs[2]), &statfs)

		ms = append(ms, &measurement{
			Header: message.Measurement(),
			EventID: EventID{
				Device: strs[2],
			},
			Properties: Properties{
				Major:     strs[0],
				Minor:     strs[1],
				TotalSize: int(statfs.Blocks) * int(statfs.Bsize),
				BlockSize: int(statfs.Bsize),
			},
			Metrics: Metrics{
				ReadOperations:  nums[0],
				ReadTime:        time.Duration(nums[3]) * time.Millisecond,
				WriteOperations: nums[4],
				WriteTime:       time.Duration(nums[7]) * time.Millisecond,
			},
		})
	}

	return
}
