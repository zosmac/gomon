// Code generated  DO NOT EDIT.
// Copyright © 2021 The Gomon Project.

package core

import (
	"os"
)

var (
	// Srcpath to strip from source file path in log messages.
	Srcpath = "/Users/keefe/Developer/gomon/gomon"

	// module identifies the import package path for this module.
	module = "github.com/zosmac/gomon"

	// executable identifies the full command path.
	executable, _ = os.Executable()

	// buildDate sets the build date for the command.
	buildDate = func() string {
		info, _ := os.Stat(executable)
		return info.ModTime().UTC().Format("2006-01-02 15:04:05 UTC")
	}()

	// vmmp is "Version, Major, Minor, Patch"
	vmmp = "v2.0.0-c0577e0f2b84"

	// Dependencies import paths and versions
	// github.com/StackExchange/wmi v1.2.1
	// github.com/beorn7/perks v1.0.1
	// github.com/cespare/xxhash/v2 v2.1.2
	// github.com/davecgh/go-spew v1.1.1
	// github.com/go-ole/go-ole v1.2.5
	// github.com/golang/protobuf v1.5.2
	// github.com/google/go-cmp v0.5.8
	// github.com/kr/pretty v0.1.0
	// github.com/kr/text v0.1.0
	// github.com/matttproud/golang_protobuf_extensions v1.0.1
	// github.com/prometheus/client_golang v1.14.0
	// github.com/prometheus/client_model v0.3.0
	// github.com/prometheus/common v0.37.0
	// github.com/prometheus/procfs v0.8.0
	// golang.org/x/net v0.4.0
	// golang.org/x/sys v0.3.0
	// google.golang.org/protobuf v1.28.1
	// gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15
	// gopkg.in/yaml.v2 v2.4.0
)
