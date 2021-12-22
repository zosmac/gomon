// Copyright © 2021 The Gomon Project.

package process

import (
	"github.com/zosmac/gomon/core"
)

var (
	h handle
)

type handle struct {
}

// open the OS process monitor.
func open() error {
	return core.Unsupported()
}

// close stops observing process events.
func (h *handle) close() {
}

// observe for events and notify observer's callbacks.
func observe() {
	core.LogInfo(core.Unsupported())
}

// userGroup determines user name and group for a running process
func (pid Pid) userGroup() (string, string, error) {
	return "", "", core.Unsupported()
}
