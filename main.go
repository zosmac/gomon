// Copyright © 2021-2023 The Gomon Project.

package main

import (
	"context"
	"fmt"
	"os"
	"path"
	"reflect"
	"strings"
	"time"

	"github.com/zosmac/gocore"
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
	gocore.Main(Main)
}

// Main called from gocore.Main.
func Main(ctx context.Context) error {
	if gocore.Flags.FlagSet.Lookup("document").Value.String() == "true" {
		message.Document()
		return nil
	}

	gocore.LogInfo("start", fmt.Errorf("pid=%d", os.Getpid()))

	if err := message.Encoder(ctx); err != nil {
		return gocore.Error("Encoder", err)
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
	serve(ctx)

	// capture and write with encoder
	ticker := flags.sample.alignTicker()
	for {
		select {
		case <-ctx.Done():
			return gocore.Error("exit "+os.Args[0], ctx.Err())

		case t := <-ticker.C:
			start := time.Now()
			last, ok := lastPrometheusCollection.Load().(time.Time)
			if !ok || prometheusSample == 0 || t.Sub(last) > 2*prometheusSample {
				ms := measure()
				message.Encode(ms)
				fmt.Fprintf(os.Stderr, "ENCODE %d measurements at %s\n\trequired %v\n",
					len(ms), start.Format(gocore.TimeFormat), time.Since(start))
			}

		case ch := <-prometheusChan:
			start := time.Now()
			var count int
			for _, m := range measure() {
				gocore.Format(
					"gomon_"+path.Base(reflect.Indirect(reflect.ValueOf(m)).Type().PkgPath()),
					"",
					reflect.ValueOf(m),
					func(name, tag string, val reflect.Value) any {
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
				count, start.Format(gocore.TimeFormat), time.Since(start))
		}
	}
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
