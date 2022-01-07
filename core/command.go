// Copyright © 2021 The Gomon Project.

package core

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
)

// go generate creates version.go to set vmmp and package dependencies for version.
//go:generate ./generate.sh

var (
	// HostName identifies the host.
	HostName, _ = os.Hostname()

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

// SignalChannel provides a channel for receiving signals.
func SignalChannel() <-chan os.Signal {
	return signalChannel()
}

// Init called by main() to initialize core.
func Init() {
}

// Main drives the show.
func Main(fn func()) {
	// set up profiling if requested
	profile()

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
		sig := <-SignalChannel()
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

	go fn()

	wg.Wait()
}

// cleanuproutines contains the functions registered for resource cleanup.
var cleanuproutines []func()

// Register enables registration of cleanup routines to run prior to exit.
func Register(fn func()) {
	cleanuproutines = append(cleanuproutines, fn)
}

// Exit calls cleanup and sets gomon's return code.
func Exit() {
	for _, fn := range cleanuproutines {
		fn()
	}
	os.Exit(exitCode)
}
