// Copyright © 2021-2023 The Gomon Project.

//go:build !windows

package file

import (
	"os"
	"syscall"

	"github.com/zosmac/gocore"
)

// owner returns the username and groupname of a file's owner.
func owner(info os.FileInfo) (string, string) {
	return gocore.Username(int(info.Sys().(*syscall.Stat_t).Uid)),
		gocore.Groupname(int(info.Sys().(*syscall.Stat_t).Gid))
}
