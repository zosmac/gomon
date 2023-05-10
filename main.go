// Copyright © 2021-2023 The Gomon Project.

package main

import (
	"context"
	"os"
	"path"
	"reflect"
	"strconv"
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
	serve(ctx)

	gocore.Error("start", nil, map[string]string{
		"pid":        strconv.Itoa(os.Getpid()),
		"command":    strings.Join(os.Args, " "),
		"executable": gocore.Executable,
		"version":    gocore.Version,
		"user":       gocore.Username(os.Getuid()),
	}).Info()

	// capture and write with encoder
	ticker := flags.sample.alignTicker()
	for {
		select {
		case <-ctx.Done():
			return gocore.Error("stop", ctx.Err(), map[string]string{
				"command": os.Args[0],
			})

		case t := <-ticker.C:
			start := time.Now()
			last, ok := lastPrometheusCollection.Load().(time.Time)
			if !ok || prometheusSample == 0 || t.Sub(last) > 2*prometheusSample {
				ms := measure()
				message.Measurements(ms)
				gocore.Error("encode", nil, map[string]string{
					"count": strconv.Itoa(len(ms)),
					"time":  time.Since(start).String(),
				}).Info()
			}

		case ch := <-prometheusChan:
			count := measures.Collections
			start := measures.CollectionTime
			for _, m := range measure() {
				gocore.Format(
					"gomon_"+path.Base(reflect.Indirect(reflect.ValueOf(m)).Type().PkgPath()),
					"",
					reflect.ValueOf(m),
					func(name, tag string, val reflect.Value) any {
						if !strings.HasPrefix(tag, "property") {
							ch <- prometheusMetric(m.ID(), name, tag, val)
							measures.Collections++
						}
						return nil
					},
				)
			}
			gocore.Error("collect", nil, map[string]string{
				"count": strconv.Itoa(measures.Collections - count),
				"time":  (measures.CollectionTime - start).String(),
			}).Info()
			prometheusDone <- struct{}{}
		}
	}
}

// measure gathers measurements of each subsystem.
func measure() (ms []message.Content) {
	start := time.Now()

	ps, pm := process.Measure()
	sm := system.Measure(ps)
	ms = append(ms, sm)
	ms = append(ms, pm...)
	ms = append(ms, io.Measure()...)
	ms = append(ms, filesystem.Measure()...)
	ms = append(ms, network.Measure()...)

	measures.CollectionTime += time.Since(start)
	measures.LokiStreams += message.LokiStreams
	ms = append(ms, &measures)

	return
}
