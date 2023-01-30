// Copyright © 2021-2023 The Gomon Project.

package file

import (
	"github.com/zosmac/gocore"
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
	gocore.Flags.Var(
		&flags.fileDirectory,
		"filedirectory",
		"[-filedirectory <path>]",
		"The `path` to the top of a directory hierarchy of files to monitor",
	)
}
