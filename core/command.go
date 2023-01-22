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

	"golang.org/x/tools/go/packages"
)

var (
	// Hostname identifies the host.
	Hostname, _ = os.Hostname()

	// executable identifies the full command path.
	executable, _ = os.Executable()

	// commandName is the base name of the executable.
	commandName = filepath.Base(executable)

	// module identifies the import package path for this module.
	// Srcpath to strip from source file path in log messages.
	module, Srcpath, vmmp = func() (mod, dir, vers string) {
		_, n, _, _ := runtime.Caller(1)
		dir = filepath.Dir(n)
		pkgs, err := packages.Load(&packages.Config{Mode: packages.NeedModule, Dir: dir})
		module := pkgs[0].Module
		mod = module.Path
		dir = module.Dir
		if err != nil || mod == "" || dir == "" {
			panic(fmt.Sprintf("go.mod not resolved %q, %v", dir, err))
		}
		_, vers, ok := strings.Cut(dir, "@")
		if ok {
			return
		}

		cmd := exec.Command("git", "show", "-s", "--format=%cI %H")
		cmd.Dir = dir
		out, err := cmd.Output()
		if err != nil {
			panic(fmt.Sprintf("git show failed %q, %v", dir, err))
		}

		tm, h, _ := strings.Cut(string(out), " ")
		t, err := time.Parse(time.RFC3339, tm)
		if err != nil {
			panic(fmt.Sprintf("time parse failed %s %v", out, err))
		}
		vers = t.UTC().Format("v0.0.0-2006010150405-") + h[:12]
		return
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
