// Copyright © 2021 The Gomon Project.

package logs

import (
	"github.com/zosmac/gomon/core"
	"github.com/zosmac/gomon/message"
)

// open obtains a watch handle for observer
func open() error {
	return core.Unsupported()
}

func observe() {
}

func report() []message.Content {
	return nil
}

// Remove exited processes' logs from observation, which is unsupported for Windows.
func Remove(pids []int) {
	core.LogInfo(core.Unsupported())
}
