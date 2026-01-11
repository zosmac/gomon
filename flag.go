// Copyright © 2021-2025 The Gomon Project.

package main

import (
	"github.com/zosmac/gocore"
)

var (
	// flags defines the command line flags.
	flags = struct {
		measurements gocore.Options
		observations gocore.Options
	}{
		measurements: gocore.Options{
			List: []string{"filesystem", "io", "network", "process", "system"},
		},
		observations: gocore.Options{
			List: []string{"file", "logs", "process"},
		},
	}
)

// init initializes the command line flags.
func init() {
	gocore.Flags.Var(
		&flags.measurements,
		"measurements",
		"-measurements", // options list added by gocore.Flags
		"A comma-separated list of `measurements` to capture and report",
	)
	gocore.Flags.Var(
		&flags.observations,
		"observations",
		"-observations", // options list added by gocore.Flags
		"A comma-separated list of `observations` to capture and report",
	)
}
