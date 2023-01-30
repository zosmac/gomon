// Copyright © 2021-2023 The Gomon Project.

package logs

import (
	"context"

	"github.com/zosmac/gocore"
	"github.com/zosmac/gomon/message"
)

// open obtains a watch handle for observer
func open() error {
	return gocore.Unsupported()
}

func observe(ctx context.Context) error {
	return nil
}

func report() []message.Content {
	return nil
}

// Remove exited processes' logs from observation, which is unsupported for Windows.
func Remove(pids []int) {
	gocore.LogInfo(gocore.Unsupported())
}
