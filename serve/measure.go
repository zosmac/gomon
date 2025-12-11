// Copyright © 2021-2023 The Gomon Project.

package serve

import (
	"context"
	"path"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/zosmac/gocore"
	"github.com/zosmac/gomon/filesystem"
	"github.com/zosmac/gomon/io"
	"github.com/zosmac/gomon/message"
	"github.com/zosmac/gomon/network"
	"github.com/zosmac/gomon/process"
	"github.com/zosmac/gomon/system"
)

func Measure(ctx context.Context, opts gocore.Options) error {

	// start the process endpoints observer (i.e. lsof)
	if slices.Contains(opts.Selected, "process") {
		if err := process.Endpoints(ctx); err != nil {
			return err
		}
	}

	// capture and write with encoder
	ticker := flags.sample.alignTicker()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case t := <-ticker:
			start := time.Now()
			last, ok := lastPrometheusCollection.Load().(time.Time)
			if !ok || prometheusSample == 0 || t.Sub(last) > 2*prometheusSample {
				ms := measure(opts)
				message.Measurements(ms)
				gocore.Error("encode", nil, map[string]string{
					"count": strconv.Itoa(len(ms)),
					"time":  time.Since(start).String(),
				}).Info()
			}

		case ch := <-prometheusChan:
			count := measures.Collections
			start := measures.CollectionTime
			for _, m := range measure(opts) {
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
func measure(opts gocore.Options) (ms []message.Content) {
	start := time.Now()

	var ps process.ProcStats
	var pm []message.Content
	var sm message.Content
	if slices.Contains(opts.Selected, "system") &&
		slices.Contains(opts.Selected, "process") {
		ps, pm = process.Measure()
		sm = system.Measure(ps)
	} else if slices.Contains(opts.Selected, "system") &&
		!slices.Contains(opts.Selected, "process") {
		ps, _ = process.Measure()
		sm = system.Measure(ps)
	} else if !slices.Contains(opts.Selected, "system") && 
		slices.Contains(opts.Selected, "process") {
		_, pm = process.Measure()
	}
	ms = append(ms, sm)
	ms = append(ms, pm...)
	if slices.Contains(opts.Selected, "io") {
		ms = append(ms, io.Measure()...)
	}
	if slices.Contains(opts.Selected, "filesystem") {
		ms = append(ms, filesystem.Measure()...)
	}
	if slices.Contains(opts.Selected, "network") {
		ms = append(ms, network.Measure()...)
	}

	measures.Header.Timestamp = start
	measures.CollectionTime += time.Since(start)
	measures.LokiStreams += message.LokiStreams
	ms = append(ms, &measures)

	return
}
