// Copyright © 2021-2023 The Gomon Project.

package serve

import (
	"errors"
	"time"

	"github.com/zosmac/gocore"
)

type (
	// sample is a command line flag type.
	sample time.Duration
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

// Set is a flag.Value interface method to enable sample as a command line flag
func (i *sample) Set(s string) error {
	d, err := time.ParseDuration(s)
	if err != nil {
		return err
	} else if d <= 0 {
		return errors.New("interval <= 0s")
	}
	*i = sample(d)
	return nil
}

// String is a flag.Value interface method to enable sample as a command line flag.
func (i *sample) String() string {
	return time.Duration(*i).String()
}

// AlignTicker aligns the sample ticking to the sample interval.
func (i sample) alignTicker() <-chan time.Time {
	ticker := make(chan time.Time)
	go func() {
		d := time.Duration(i)
		for {
			t := time.Now()
			next := d - t.Sub(t.Truncate(d))
			if next < d/4 { // don't let ticker tick too soon
				next += d
			}
			ticker <- <-time.After(next)
		}
	}()
	return ticker
}
