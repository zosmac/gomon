// Copyright © 2021-2023 The Gomon Project.

package process

import (
	"github.com/zosmac/gocore"
)

var (
	h handle
)

type handle struct {
}

// open the OS process monitor.
func open() error {
	return gocore.Unsupported()
}

// close stops observing process events.
func (h *handle) close() {
}

// observe for events and notify observer's callbacks.
func observe() error {
	return gocore.Unsupported()
}

// userGroup determines user name and group for a running process
func (pid Pid) userGroup() (string, string, error) {
	return "", "", gocore.Unsupported()
}
