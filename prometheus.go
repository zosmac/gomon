// Copyright © 2021-2023 The Gomon Project.

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/zosmac/gocore"

	"gopkg.in/yaml.v3"
)

type (
	// prometheusCollector complies with the Prometheus Collector interface.
	prometheusCollector struct{}

	// prometheusJson defines the prometheus configuration query response envelope.
	prometheusJson struct {
		Status string `json:"status"`
		Data   struct {
			Yaml string `json:"yaml"` // []byte type unmarshals as base-64 :(
		} `json:"data"`
	}

	// prometheusYaml defines the prometheus configuration query response content.
	prometheusYaml struct {
		Global struct {
			ScrapeInterval string `yaml:"scrape_interval"`
		} `yaml:"global"`
		ScrapeConfigs []struct {
			Jobname        string `yaml:"job_name"`
			ScrapeInterval string `yaml:"scrape_interval"`
		} `yaml:"scrape_configs"`
	}
)

var (
	// descs maps metric names to descriptions.
	descs = map[string]*prometheus.Desc{}

	// lastPrometheusCollection functions as a dead man's switch.
	lastPrometheusCollection atomic.Value

	// prometheusConfigRequest is the REST query to retrieve the configuration.
	prometheusConfigRequest = http.Request{
		Method: http.MethodGet,
		URL: &url.URL{
			Scheme: "http",
			Host:   "localhost:9090",
			Path:   "/api/v1/status/config",
		},
	}

	// prometheusSample is the configured duration between prometheus samples.
	prometheusSample time.Duration

	// prometheusChan passes the Collect channel to main's measure loop.
	prometheusChan = make(chan chan<- prometheus.Metric, 1)

	// prometheusDone signals that main's measure is complete.
	prometheusDone = make(chan struct{}, 1)
)

// Describe returns metric descriptions for prometheusCollector.
// This is irrelevant as Collect() uses prometheus.MustNewConstMetric
func (c *prometheusCollector) Describe(ch chan<- *prometheus.Desc) {
	// prometheus.DescribeByCollect(c, ch)
}

// Collect returns the current state of all metrics to Prometheus.
func (c *prometheusCollector) Collect(ch chan<- prometheus.Metric) {
	lastPrometheusCollection.Store(time.Now())
	prometheusChan <- ch
	<-prometheusDone

	if prometheusSample == 0 {
		var err error
		prometheusSample, err = scrapeInterval()
		if err != nil {
			gocore.Error("prometheus config", err).Err()
		}
	}
}

// prometheusMetric complies with Formatter function prototype for encoding metrics as Prometheus metrics.
func prometheusMetric(id string, name, tag string, val reflect.Value) prometheus.Metric {
	var metric float64
	var property string

	switch v := val.Interface().(type) {
	case time.Time:
		if v.IsZero() {
			metric = 0.0
		} else {
			metric = float64(v.UnixNano()) / 1e9 // convert to seconds
			property = v.Format(gocore.RFC3339Milli)
		}
	case int, int8, int16, int32, int64:
		metric = float64(val.Int())
	case time.Duration:
		metric = float64(val.Int()) / 1e9
	case uint, uint8, uint16, uint32, uint64:
		metric = float64(val.Uint())
	case float32, float64:
		metric = val.Float()
	case fmt.Stringer:
		property = v.String()
	default:
		property = fmt.Sprintf("%v", v)
	}

	s := strings.Split(tag, ",")
	t := "counter"
	u := "none"
	if len(s) > 0 {
		t = s[0]
	}
	if len(s) > 1 {
		u = s[1]
	}

	switch u {
	case "ns":
		name += "_seconds"
	case "B":
		name += "_bytes"
	}

	var valueType prometheus.ValueType
	switch t {
	case "counter":
		valueType = prometheus.CounterValue
	case "gauge":
		valueType = prometheus.GaugeValue
	default:
		valueType = prometheus.UntypedValue
	}

	if t == "property" {
		desc, ok := descs[name]
		if !ok {
			l := strings.SplitN(name, "_", 3) // pull out source
			desc = prometheus.NewDesc(name, "property", []string{"id", "value"},
				prometheus.Labels{
					"source": l[1],
				})
			descs[name] = desc
		}

		return prometheus.MustNewConstMetric(desc, valueType, 0.0, id, property)
	}

	desc, ok := descs[name]
	if !ok {
		l := strings.SplitN(name, "_", 3) // pull out source
		desc = prometheus.NewDesc(name, "units: "+u, []string{"id"},
			prometheus.Labels{
				"source": l[1],
			})
		descs[name] = desc
	}

	return prometheus.MustNewConstMetric(desc, valueType, metric, id)
}

// scrapeInterval asks Prometheus for the scrape interval it will query gomon for metrics.
func scrapeInterval() (time.Duration, error) {
	resp, err := http.DefaultClient.Do(&prometheusConfigRequest)
	if err != nil {
		return 0, gocore.Error("prometheus query", err)
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return 0, gocore.Error("prometheus query", err)
	}

	jsn := prometheusJson{}
	if err := json.Unmarshal(body, &jsn); err != nil || jsn.Status != "success" {
		return 0, gocore.Error("prometheus query "+jsn.Status, err)
	}

	yml := prometheusYaml{}
	if err := yaml.Unmarshal([]byte(jsn.Data.Yaml), &yml); err != nil {
		return 0, gocore.Error("prometheus yaml", err)
	}

	for _, config := range yml.ScrapeConfigs {
		if config.Jobname == "gomon" {
			return time.ParseDuration(config.ScrapeInterval)
		}
	}

	return time.ParseDuration(yml.Global.ScrapeInterval)
}
