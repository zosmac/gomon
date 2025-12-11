// Copyright © 2021-2025 The Gomon Project.

package main

import (
	"github.com/zosmac/gocore"
)

var (
	// flags defines the command line flags.
	flags = struct {
		measures gocore.Options
		events gocore.Options
	}{
		measures: gocore.Options{
			List: []string{"filesystem", "io", "network", "process", "system"},
		},
		events: gocore.Options{
			List: []string{"file", "logs", "process"},
		},
	}
)

// init initializes the command line flags.
func init() {
	gocore.Flags.Var(
		&flags.events,
		"events",
		"-events", // options list added by gocore.Flags
		"A comma-separated list of `events` to capture and report",
	)

	gocore.Flags.Var(
		&flags.measures,
		"measures",
		"-measures", // options list added by gocore.Flags
		"A comma-separated list of `measures` to capture and report",
	)
}
