// Copyright © 2021-2023 The Gomon Project.

package main

import (
	"context"
	"flag"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/zosmac/gocore"
	"github.com/zosmac/gomon/file"
	"github.com/zosmac/gomon/logs"
	"github.com/zosmac/gomon/message"
	"github.com/zosmac/gomon/process"
	"github.com/zosmac/gomon/serve"
)

// main
func main() {
	gocore.Main(Main)
}

// Main called from gocore.Main.
func Main(ctx context.Context) error {
	if gocore.Flags.FlagSet.Lookup("document").Value.String() == "true" {
		message.Document()
		return nil
	}

	if err := message.Encoder(ctx); err != nil {
		return gocore.Error("encoder", err)
	}

	if slices.Contains(flags.observations.Selected, "logs") {
		if err := logs.Observer(ctx); err != nil {
			return gocore.Error("logs Observer", err)
		}
	}

	if slices.Contains(flags.observations.Selected, "file") {
		if err := file.Observer(ctx); err != nil {
			return gocore.Error("files Observer", err)
		}
	}

	if slices.Contains(flags.observations.Selected, "process") {
		if err := process.Observer(ctx); err != nil {
			return gocore.Error("processes Observer", err)
		}
	}

	// fire up the http server
	serve.Serve(ctx)

	executable, _ := os.Executable()
	settings := map[string]string{
		"pid":        strconv.Itoa(os.Getpid()),
		"command":    strings.Join(os.Args, " "),
		"executable": executable,
		"version":    gocore.Version,
		"user":       gocore.Username(os.Getuid()),
	}
	gocore.Flags.FlagSet.VisitAll(func(f *flag.Flag) {
		settings[f.Name] = f.Value.String()
	})

	gocore.Error("start", nil, settings).Info()

	if slices.Contains(flags.measurements.Selected, "none") {
		<-ctx.Done()
		return gocore.Error("stop", ctx.Err(), map[string]string{"command": os.Args[0]})
	}

	return gocore.Error("stop", serve.Measure(ctx, flags.measurements), map[string]string{
		"command": os.Args[0],
	})
}
