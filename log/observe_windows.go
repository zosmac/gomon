// Copyright © 2021 The Gomon Project.

package log

import (
	"errors"
	"runtime"

	"github.com/zosmac/gomon/core"
	"github.com/zosmac/gomon/message"
)

// open obtains a watch handle for observer
func open() error {
	return core.NewError(runtime.GOOS, errors.New("unsupported"))
}

func listen() {
}

func report() []message.Content {
	return nil
}

// Remove exited processes' logs from observation, which is unsupported for Windows.
func Remove(pids []int) {
	core.LogError(core.NewError(runtime.GOOS, errors.New("unsupported")))
}
