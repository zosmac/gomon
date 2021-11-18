// Copyright © 2021 The Gomon Project.

package core

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
)

// profile turns on CPU or Memory performance profiling of command.
// Profiling can also be enabled via the /debug/pprof endpoint.
func profile() {
	for i, s := range os.Args {
		if s == "-cpuprofile" {
			os.Args = append(os.Args[:i], os.Args[i+1:]...) // remove -cpuprofile flag
			f, err := os.CreateTemp("", "pprof_")
			if err != nil {
				LogError(errors.New("-cpuprofile " + err.Error()))
			}
			pprof.StartCPUProfile(f)
			Register(func() {
				cmd, _ := os.Executable()
				fmt.Fprintf(os.Stderr, "CPU profile written to %[1]q.\nUse the following command to evaluate:\n"+
					"\033[1;31mgo tool pprof -web %[2]s %[1]s\033[0m\n", f.Name(), cmd)
				pprof.StopCPUProfile()
				f.Close()
			})
		}
	}

	for i, s := range os.Args {
		if s == "-memprofile" {
			os.Args = append(os.Args[:i], os.Args[i+1:]...) // remove -memprofile flag
			f, err := os.CreateTemp(".", "mprof_")
			if err != nil {
				LogError(errors.New("-memprofile " + err.Error()))
			}
			Register(func() {
				runtime.GC()
				pprof.WriteHeapProfile(f)
				f.Close()
				cmd, _ := os.Executable()
				fmt.Fprintf(os.Stderr, "Memory profile written to %[1]q.\nUse the following command to evaluate:\n"+
					"\033[1;31mgo tool pprof -web %[2]s %[1]s\033[0m\n", f.Name(), cmd)
			})
		}
	}
}
