// Copyright © 2021 The Gomon Project.

package process

import (
	"strconv"
	"time"

	"github.com/zosmac/gomon/message"
)

// Pid is the identifier for a process.
type Pid int

func (pid Pid) String() string {
	return strconv.Itoa(int(pid))
}

const (
	// message sources.
	sourceProcess processSource = "process"
)

var (
	// processSources valid source values for messages.
	processSources = message.ValidValues{
		sourceProcess,
	}
)

type (
	//processSource type.
	processSource string

	// id identifies the message.
	id struct {
		ppid      Pid       // for observer
		Name      string    `json:"name" gomon:"property"`
		Pid       Pid       `json:"pid" gomon:"property"`
		Starttime time.Time `json:"starttime" gomon:"property"`
	}
)
