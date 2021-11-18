// Copyright © 2021 The Gomon Project.

package filesystem

import (
	"time"

	"github.com/zosmac/gomon/core"
	"github.com/zosmac/gomon/message"
)

// Measure captures system's filesystems' metrics.
func Measure() []message.Content {
	qs, err := filesystems()
	if err != nil {
		core.LogError(err)
		return nil
	}

	return message.Gather(qs, 5*time.Second)
}
