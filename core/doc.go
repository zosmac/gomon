// Copyright © 2021 The Gomon Project.

/*
Package core implements functionality used by the "gomon" command. Functions support
 * a server command line framework
 * extensions to the Go flag package
 * help for command syntax
 * documentation of command output
 * enhanced logging

The core package defines the following command line flags:
 * -version:  to report the current version of Gomon
 * -document: to document the output that Gomon produces
 * -port:     to set the port for requesting the process connections nodegraph, for Prometheus metric collection, and for profiling (default 1234)
 * -sample:   to specify the sampling interval for measurements (default 15s)
*/
package core
