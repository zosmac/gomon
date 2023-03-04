// Copyright © 2021-2023 The Gomon Project.

package logs

import (
	"github.com/zosmac/gocore"
	"github.com/zosmac/gomon/message"
)

// open obtains a watch handle for observer.
func open() error {
	return gocore.Unsupported()
}

// close OS resources.
func close() {
}

func observe() error {
	return gocore.Unsupported()
}

func report() []message.Content {
	return nil
}

// Remove exited processes' logs from observation, which is unsupported for Windows.
func Remove(pids []int) {
}
