// Copyright © 2021-2023 The Gomon Project.

package main

import (
	"context"
	"os"
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

	if err := logs.Observer(ctx); err != nil {
		return gocore.Error("logs Observer", err)
	}

	if err := file.Observer(ctx); err != nil {
		return gocore.Error("files Observer", err)
	}

	if err := process.Observer(ctx); err != nil {
		return gocore.Error("processes Observer", err)
	}

	// fire up the http server
	serve.Serve(ctx)

	gocore.Error("start", nil, map[string]string{
		"pid":        strconv.Itoa(os.Getpid()),
		"command":    strings.Join(os.Args, " "),
		"executable": gocore.Executable,
		"version":    gocore.Version,
		"user":       gocore.Username(os.Getuid()),
	}).Info()

	return gocore.Error("stop", serve.Measure(ctx), map[string]string{
		"command": os.Args[0],
	})
}
