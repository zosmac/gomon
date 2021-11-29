// Copyright © 2021 The Gomon Project.

//go:build !windows

package file

import (
	"os"
	"syscall"

	"github.com/zosmac/gomon/core"
)

// owner returns the username and groupname of a file's owner.
func owner(info os.FileInfo) (string, string) {
	return core.Username(int(info.Sys().(*syscall.Stat_t).Uid)),
		core.Groupname(int(info.Sys().(*syscall.Stat_t).Gid))
}
