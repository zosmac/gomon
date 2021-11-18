// Copyright © 2021 The Gomon Project.

package file

import (
	"github.com/zosmac/gomon/core"
)

var (
	// flags defines the command line flags.
	flags = struct {
		fileDirectory string
	}{
		fileDirectory: "/",
	}
)

// init initializes the command line flags.
func init() {
	core.Flags.Var(&flags.fileDirectory, "filedirectory", "[-filedirectory <path>]",
		"The `path` to the top of a directory hierarchy of files to monitor")
}
