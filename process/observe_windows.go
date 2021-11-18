// Copyright © 2021 The Gomon Project.

package process

import (
	"errors"
	"fmt"
	"runtime"

	"github.com/zosmac/gomon/core"
)

var (
	h handle
)

type handle struct {
}

// open the OS process monitor.
func open() error {
	return core.NewError(runtime.GOOS, errors.New("unsupported"))
}

// close stops listening for process events.
func (h *handle) close() {
}

// listen for events and notify observer's callbacks.
func listen() {
	core.LogError(fmt.Errorf("%s unsupported", runtime.GOOS))
}

// userGroup determines user name and group for a running process
func (pid Pid) userGroup() (string, string, error) {
	return "", "", core.NewError(runtime.GOOS, errors.New("unsupported"))
}
