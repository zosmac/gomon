// Copyright © 2021-2023 The Gomon Project.

package serve

import (
	"context"
	"path"
	"reflect"
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

func Measure(ctx context.Context) error {
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
