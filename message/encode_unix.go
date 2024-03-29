// Copyright © 2021-2023 The Gomon Project.

//go:build !windows

package message

import (
	"io/fs"
	"os"
	"syscall"
)

func chown(f *os.File, info fs.FileInfo) {
	f.Chown(int(info.Sys().(*syscall.Stat_t).Uid), int(info.Sys().(*syscall.Stat_t).Gid))
}
