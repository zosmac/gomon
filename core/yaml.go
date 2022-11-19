// Copyright © 2021 The Gomon Project.

package core

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v2"
)

// ValueYaml uses the keypath to extract the nested element from a yaml configuration.
func ValueYaml(keypath []string, y interface{}) string {
	return strings.TrimSpace(valueYaml(keypath, y))
}

// valueYaml uses the keypath to extract the nested element from a yaml configuration.
func valueYaml(keypath []string, y interface{}) string {
	if len(keypath) == 0 {
		return decodeYaml("", y)
	}
	var is []interface{}
	switch y := y.(type) {
	default:
		return fmt.Sprint(y)
	case yaml.MapItem:
		if keypath[0] == y.Key {
			return decodeYaml("", y.Value)
		}
	case []interface{}:
		is = y
	case yaml.MapSlice:
		for _, m := range y {
			is = append(is, m)
		}
	}

	for _, i := range is {
		switch i := i.(type) {
		case yaml.MapSlice:
			if keypath[0] == i[0].Key {
				if len(keypath) == 1 {
					return decodeYaml("", is)
				}
				if v, ok := i[0].Value.(string); ok {
					if keypath[1] == v {
						return valueYaml(keypath[2:], i)
					}
				} else {
					return valueYaml(keypath[1:], i)
				}
			}
		case yaml.MapItem:
			if keypath[0] == i.Key {
				if _, ok := i.Value.(string); ok && len(keypath) > 1 {
					return valueYaml(keypath[1:], i)
				}
				return valueYaml(keypath[1:], i.Value)
			}
		}
	}
	return ""
}

// decodeYaml decodes the yaml object value
func decodeYaml(indent string, y interface{}) string {
	var s string
	switch y := y.(type) {
	case yaml.MapSlice:
		s = "\n"
		for _, i := range y {
			if len(indent) > 1 && indent[len(indent)-2] == '-' {
				s += indent[2:]
				indent = indent[:len(indent)-2]
			} else {
				s += indent
			}
			s += i.Key.(string) + ":"
			s += decodeYaml(indent+"  ", i.Value)
		}
	case []interface{}:
		if len(y) == 0 {
			return " []\n"
		}
		for _, i := range y {
			s += decodeYaml(indent+"- ", i)
		}
	default:
		if len(indent) > 1 && indent[len(indent)-2] == '-' {
			return fmt.Sprintf("\n%s%s", indent[2:], y)
		}
		return fmt.Sprintf(" %v\n", y)
	}
	return s
}

// y is an example prometheus.yml file to test decoding.
/*
var y = []byte(`
global:
  scrape_interval: 15s
  scrape_timeout: 10s
  evaluation_interval: 15s
alerting:
  alertmanagers:
  - static_configs:
    - targets: []
    scheme: http
    timeout: 10s
    api_version: v1
scrape_configs:
- job_name: prometheus
  honor_timestamps: true
  scrape_interval: 15s
  scrape_timeout: 10s
  metrics_path: /metrics
  scheme: http
  static_configs:
  - targets:
    - localhost:9090
- job_name: grafana
  honor_timestamps: true
  scrape_interval: 15s
  scrape_timeout: 10s
  metrics_path: /metrics
  scheme: http
  static_configs:
  - targets:
    - localhost:3000
    - localhost:7777
- job_name: gomon
  honor_timestamps: true
  scrape_interval: 15s
  scrape_timeout: 10s
  metrics_path: /metrics
  scheme: http
  static_configs:
  - targets:
    - localhost:1234
`)

// test searches various keypaths for values in prometheus.yml.
func test() {
	var keypath []string
	var val string
	var dur time.Duration

	keypath = []string{"scrape_configs"}
	val = ValueYaml(keypath, y)
	fmt.Fprintf(os.Stderr, "keypath %v value %s\n", keypath, val)

	keypath = []string{"scrape_configs", "job_name"}
	val = ValueYaml(keypath, y)
	fmt.Fprintf(os.Stderr, "keypath %v value %s\n", keypath, val)

	keypath = []string{"scrape_configs", "job_name", "gomon"}
	val = ValueYaml(keypath, y)
	fmt.Fprintf(os.Stderr, "keypath %v value %s\n", keypath, val)

	keypath = []string{"scrape_configs", "job_name", "grafana", "static_configs"}
	val = ValueYaml(keypath, y)
	fmt.Fprintf(os.Stderr, "keypath %v value %s\n", keypath, val)

	keypath = []string{"alerting", "alertmanagers", "static_configs", "scheme"}
	val = ValueYaml(keypath, y)
	fmt.Fprintf(os.Stderr, "keypath %v value %s\n", keypath, val)

	keypath = []string{"global", "scrape_interval"}
	val = ValueYaml(keypath, y)
	fmt.Fprintf(os.Stderr, "keypath %v value %s\n", keypath, val)
	dur, _ = time.ParseDuration(strings.TrimSpace(val))
	fmt.Fprintf(os.Stderr, "default scrape_interval is %v\n", dur)

	keypath = []string{"scrape_configs", "job_name", "gomon", "scrape_interval"}
	val = ValueYaml(keypath, y)
	fmt.Fprintf(os.Stderr, "keypath %v value %s\n", keypath, val)
	dur, _ = time.ParseDuration(strings.TrimSpace(val))
	fmt.Fprintf(os.Stderr, "gomon scrape_interval is %v\n", dur)
}
*/
