// Copyright © 2021-2023 The Gomon Project.

/*
Package main implements the Go language "gomon" system monitor command. Additional functionality includes
  - an HTTP server
  - delivery of metrics to Prometheus
  - delivery of logs to Loki
  - production of inter-process connections node graph with Graphviz

The main package defines the following command line flags:
  - -port:   to set the port for requesting the process connections nodegraph, for Prometheus metric collection, Loki log collection, and for profiling (default 1234)
  - -sample: to specify the sampling interval for measurements (default 15s)
*/
package main
