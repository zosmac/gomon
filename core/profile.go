// Copyright © 2021 The Gomon Project.

package core

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
)

// profile turns on CPU or Memory performance profiling of command.
// Profiling can also be enabled via the /debug/pprof endpoint.
func profile(ctx context.Context) {
	for i, s := range os.Args {
		if s == "-cpuprofile" {
			os.Args = append(os.Args[:i], os.Args[i+1:]...) // remove -cpuprofile flag
			if f, err := os.CreateTemp("", "pprof_"); err != nil {
				LogError(Error("-cpuprofile", err))
			} else {
				go func() {
					pprof.StartCPUProfile(f)
					<-ctx.Done()
					pprof.StopCPUProfile()
					cmd, _ := os.Executable()
					fmt.Fprintf(os.Stderr,
						"CPU profile written to %[1]q.\nUse the following command to evaluate:\n"+
							"\033[1;31mgo tool pprof -web %[2]s %[1]s\033[0m\n",
						f.Name(),
						cmd,
					)
					f.Close()
				}()
				break
			}
		}
	}

	for i, s := range os.Args {
		if s == "-memprofile" {
			os.Args = append(os.Args[:i], os.Args[i+1:]...) // remove -memprofile flag
			if f, err := os.CreateTemp(".", "mprof_"); err != nil {
				LogError(Error("-memprofile", err))
			} else {
				go func() {
					<-ctx.Done()
					runtime.GC()
					pprof.WriteHeapProfile(f)
					cmd, _ := os.Executable()
					fmt.Fprintf(os.Stderr,
						"Memory profile written to %[1]q.\nUse the following command to evaluate:\n"+
							"\033[1;31mgo tool pprof -web %[2]s %[1]s\033[0m\n",
						f.Name(),
						cmd,
					)
					f.Close()
				}()
				break
			}
		}
	}
}
