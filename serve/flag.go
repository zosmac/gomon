// Copyright © 2021-2023 The Gomon Project.

package serve

import (
	"errors"
	"time"

	"github.com/zosmac/gocore"
)

var (
	// flags defines the command line flags.
	flags = struct {
		port int
		sample
	}{
		port:   1234,
		sample: sample(15 * time.Second),
	}
)

// init initializes the command line flags.
func init() {
	gocore.Flags.Var(
		&flags.port,
		"port",
		"[-port n]",
		"Port number for Gomon REST server",
	)

	if flags.sample < sample(time.Second) {
		flags.sample = sample(time.Second)
	}
	gocore.Flags.Var(
		&flags.sample,
		"sample",
		"[-sample <interval>]",
		"Sample metrics at `interval`, specified in Go time.Duration string format",
	)

	gocore.Flags.CommandDescription = `Monitors the local host,
	measuring state and usage of:
		• system cpu
		• system memory
		• filesystems
		• I/O devices
		• network interfaces
		• processes
	observing changes to:
		• files
		• logs
		• processes`
}

// sample is a command line flag type.
type sample time.Duration

// Set is a flag.Value interface method to enable sample as a command line flag
func (i *sample) Set(s string) error {
	d, err := time.ParseDuration(s)
	if d <= 0 {
		return errors.New("invalid sample interval")
	}
	*i = sample(d)
	return err
}

// String is a flag.Value interface method to enable sample as a command line flag.
func (i *sample) String() string {
	return time.Duration(*i).String()
}

// AlignTicker aligns the sample ticking.
func (i sample) alignTicker() *time.Ticker {
	d := time.Duration(i)
	t := time.Now()
	<-time.After(d - t.Sub(t.Truncate(d)))
	return time.NewTicker(d)
}
