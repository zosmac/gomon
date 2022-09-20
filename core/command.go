// Copyright © 2021 The Gomon Project.

package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"
)

// go generate creates version.go to set vmmp and package dependencies for version.
//go:generate ./generate.sh

var (
	// Context is canceled on exit.
	Context context.Context
	Cancel  context.CancelFunc

	// Hostname identifies the host.
	Hostname, _ = os.Hostname()

	// executable identifies the full command path.
	executable, _ = os.Executable()

	// buildDate sets the build date for the command.
	buildDate = func() string {
		info, _ := os.Stat(executable)
		return info.ModTime().UTC().Format("2006-01-02 15:04:05 UTC")
	}()

	// commandName is the base name of the executable.
	commandName = filepath.Base(executable)

	// Document is set by the message package to prevent import recursion.
	Document func()
)

// version returns the command's version information.
// vmmp contains the version string in the generated version.go.
// Initialize these by running go generate ./... prior to building the gomon command.
func version() {
	fmt.Fprintf(os.Stderr,
		`Command    - %s
Version    - %s
Build Date - %s
Compiler   - %s %s_%s
Copyright © 2021 The Gomon Project.
`,
		executable, vmmp, buildDate, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}

// Init called by main() to initialize core.
func Init() {
	Context, Cancel = context.WithCancel(context.Background())
}

// Main drives the show.
func Main(fn func(context.Context)) {
	// set up profiling if requested
	profile(Context)

	if !parse(os.Args[1:]) {
		return
	}

	if Flags.version {
		version()
		return
	}

	if Flags.document {
		Document()
		return
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		sig := <-signalChannel()
		LogError(fmt.Errorf("signal %[1]d (%[1]s) pid %d", sig, os.Getpid()))
		switch sig := sig.(type) {
		case syscall.Signal:
			switch sig {
			case syscall.SIGSEGV:
				buf := make([]byte, 16384)
				n := runtime.Stack(buf, true)
				fmt.Fprintln(os.Stderr, string(buf[:n]))
			default:
			}
		}
	}()

	go fn(Context)

	wg.Wait()
}

// Exit calls cleanup and sets gomon's return code.
func Exit() {
	Cancel() // signal all service routines to cleanup and exit

	<-time.After(2 * time.Second) // wait a bit for all resource cleanup to complete

	os.Exit(exitCode)
}
