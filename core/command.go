// Copyright © 2021 The Gomon Project.

package core

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
)

// go generate creates version.go to set vmmp and package dependencies for version.
//go:generate ./generate.sh

var (
	// Hostname identifies the host.
	Hostname, _ = os.Hostname()

	// executable identifies the full command path.
	executable, _ = os.Executable()

	// commandName is the base name of the executable.
	commandName = filepath.Base(executable)

	// module identifies the import package path for this module.
	// Srcpath to strip from source file path in log messages.
	module, Srcpath = func() (string, string) {
		cmd := exec.Command("go", "list", "-m", "-f", "{{.Path}}\n{{.Dir}}")
		_, n, _, _ := runtime.Caller(1)
		fmt.Fprintf(os.Stderr, "depth 1 name %s\n", n)
		cmd.Dir = filepath.Dir(n)
		if out, err := cmd.Output(); err == nil {
			mod, dir, _ := strings.Cut(string(out), "\n")
			dir, _, _ = strings.Cut(dir, "@")
			if mod != "" && dir != "" {
				return strings.TrimSpace(mod), strings.TrimSpace(dir)
			}
		}
		panic(fmt.Sprintf("no go.mod found from build directory %q", cmd.Dir))
	}()

	// buildDate sets the build date for the command.
	buildDate = func() string {
		info, _ := os.Stat(executable)
		return info.ModTime().UTC().Format("2006-01-02 15:04:05 UTC")
	}()

	// Document is set by the message package to prevent import recursion.
	Document func()
)

// version returns the command's version information.
// vmmp contains the version string in the generated version.go.
// Initialize these by running go generate ./... prior to building the gomon command.
func version() {
	fmt.Fprintf(os.Stderr,
		`Command    - %s
Module     - %s
Version    - %s
Build Date - %s
Compiler   - %s %s_%s
Copyright © 2021 The Gomon Project.
`,
		executable, module, vmmp, buildDate, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}

// Main drives the show.
func Main(fn func(context.Context)) {
	ctx, cncl := context.WithCancel(context.Background())
	defer cncl()

	// set up profiling if requested
	profile(ctx)

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
			cncl()                    // signal all service routines to cleanup and exit
			<-time.After(time.Second) // wait a bit for all resource cleanup to complete
			os.Exit(exitCode)
		}
	}()

	go fn(ctx)

	// run osEnvironment on main thread for the native host application environment setup (e.g. MacOS main run loop)
	osEnvironment(ctx)

	wg.Wait()
}
