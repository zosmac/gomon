// Copyright © 2021-2023 The Gomon Project.

package process

import (
	"github.com/zosmac/gocore"
)

var (
	// flags defines the command line flags.
	flags = struct {
		top uint
	}{
		top: 20,
	}
)

// init initializes the command line flags.
func init() {
	gocore.Flags.Var(
		&flags.top,
		"top",
		"[-top <count>]",
		"The `count` to report of processes consuming most CPU time",
	)
}
