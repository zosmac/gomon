// Copyright © 2021-2023 The Gomon Project.

package message

import (
	"time"

	"github.com/zosmac/gocore"
)

var (
	// flags defines the command line flags.
	flags = struct {
		document bool
		pretty   bool
		rotate
	}{
		rotate: rotate{interval: 0 * time.Hour},
	}
)

type (
	// rotate is a command line flag type.
	rotate struct {
		interval time.Duration
		format   string
	}
)

// Set is a flag.Value interface method to enable rotate as a command line flag.
func (r *rotate) Set(s string) error {
	d, err := time.ParseDuration(s)
	if err != nil {
		return gocore.Error("ParseDuration", err)
	}
	if d > 0 && d < time.Second {
		d = time.Second
	}
	r.interval = d
	r.format = "20060102" // day resolution
	if d < 24*time.Hour {
		r.format += "15" // hour resolution
	}
	if d < time.Hour {
		r.format += "04" // minute resolution
	}
	if d < time.Minute {
		r.format += "05" // second resolution
	}
	r.format += "Z"

	return nil
}

// String is a flag.Value interface method to enable rotate as a command line flag.
func (r *rotate) String() string {
	return r.interval.String()
}

// init initializes the command line flags.
func init() {
	gocore.Flags.Var(
		&flags.document,
		"document",
		"[-document]",
		"Document the measurements and observations and exit",
	)
	gocore.Flags.Var(
		&flags.pretty,
		"pretty",
		"[-pretty]",
		"Produce output in human readable format",
	)

	var def string
	if flags.rotate.interval == 0 {
		def = " (default do not rotate)"
	} else if flags.rotate.interval < time.Second {
		flags.rotate.interval = time.Second
	}
	flags.rotate.Set(flags.rotate.interval.String())
	gocore.Flags.Var(
		&flags.rotate,
		"rotate",
		"[-rotate <interval>]",
		"Rotate output file at `interval`, specified in Go time.Duration string format"+def,
	)
}
