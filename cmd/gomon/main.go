// Copyright © 2021 The Gomon Project.

package main

import (
	"fmt"
	"os"
	"path"
	"reflect"
	"strings"
	"time"

	"github.com/zosmac/gomon/core"
	"github.com/zosmac/gomon/file"
	"github.com/zosmac/gomon/filesystem"
	"github.com/zosmac/gomon/io"
	"github.com/zosmac/gomon/logs"
	"github.com/zosmac/gomon/message"
	"github.com/zosmac/gomon/network"
	"github.com/zosmac/gomon/process"
	"github.com/zosmac/gomon/system"
)

// main
func main() {
	core.Init()
	core.Main(Main)
	core.Exit()
}

// Main called from core.Main.
func Main() {
	cmd, err := os.Executable()
	if err != nil {
		cmd = os.Args[0]
	}
	if len(os.Args) > 1 {
		cmd += " " + strings.Join(os.Args[1:], " ")
	}
	core.LogInfo(fmt.Errorf("start [%d] %q", os.Getpid(), cmd))

	if err := message.Encoder(); err != nil {
		core.LogError(err)
		return
	}

	if err := logs.Observer(); err != nil {
		core.LogError(err)
		return
	}

	if err := file.Observer(); err != nil {
		core.LogError(err)
		return
	}

	if err := process.Observer(); err != nil {
		core.LogError(err)
		return
	}

	// capture and write with encoder
	go func() {
		ticker := core.Flags.Sample.AlignTicker()
		for {
			select {
			case t := <-ticker.C:
				start := time.Now()
				last, ok := lastPrometheusCollection.Load().(time.Time)
				if !ok || t.Sub(last) > time.Duration(2*core.Flags.Sample) {
					ms := measure()
					message.Encode(ms)
					fmt.Fprintf(os.Stderr, "ENCODE %d measurements at %s\n\trequired %v\n",
						len(ms), start.Format(core.TimeFormat), time.Since(start))
				}

			case ch := <-prometheusChan:
				start := time.Now()
				var count int
				for _, m := range measure() {
					core.Format(
						"gomon_"+path.Base(reflect.Indirect(reflect.ValueOf(m)).Type().PkgPath()),
						"",
						reflect.ValueOf(m),
						func(name, tag string, val reflect.Value) interface{} {
							if !strings.HasPrefix(tag, "property") {
								ch <- prometheusMetric(m, name, tag, val)
								count++
							}
							return nil
						},
					)
				}
				prometheusDone <- struct{}{}
				fmt.Fprintf(os.Stderr, "COLLECT %d metrics at %s\n\trequired %v\n",
					count, start.Format(core.TimeFormat), time.Since(start))
			}
		}
	}()

	// fire up the http server
	serve()
}

// measure gathers measurements of each subsystem.
func measure() (ms []message.Content) {
	ps, pm := process.Measure()
	sm := system.Measure(ps)
	ms = append(ms, sm)
	ms = append(ms, pm...)
	ms = append(ms, io.Measure()...)
	ms = append(ms, filesystem.Measure()...)
	ms = append(ms, network.Measure()...)
	return
}
