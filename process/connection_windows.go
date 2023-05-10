// Copyright © 2021-2023 The Gomon Project.

package process

import (
	"context"

	"github.com/zosmac/gocore"
)

// lsofCommand starts the lsof command to capture process connections
func lsofCommand(ready chan<- struct{}) {
	ready <- struct{}{}
}

// Endpoints starts the lsof command to capture process connections.
func Endpoints(_ context.Context) error {
	return gocore.Unsupported()
}
