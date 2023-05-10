// Copyright © 2021-2023 The Gomon Project.

package filesystem

import (
	"time"

	"github.com/zosmac/gocore"
	"github.com/zosmac/gomon/message"
)

// Measure captures system's filesystems' metrics.
func Measure() []message.Content {
	qs, err := filesystems()
	if err != nil {
		gocore.Error("filesystems", err).Err()
		return nil
	}

	return message.Gather(qs, 5*time.Second)
}
