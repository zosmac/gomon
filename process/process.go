// Copyright © 2021 The Gomon Project.

package process

import (
	"strconv"
	"time"
)

// Pid is the identifier for a process.
type Pid int

func (pid Pid) String() string {
	return strconv.Itoa(int(pid))
}

type (
	// Id identifies the message.
	Id struct {
		ppid      Pid       // for observer
		Name      string    `json:"name" gomon:"property"`
		Pid       Pid       `json:"pid" gomon:"property"`
		Starttime time.Time `json:"starttime" gomon:"property"`
	}
)
