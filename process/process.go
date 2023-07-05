// Copyright © 2021-2023 The Gomon Project.

package process

import (
	"strconv"
	"time"
)

type (
	// Pid is the identifier for a process.
	Pid int

	// Id identifies the message.
	Id struct {
		ppid      Pid       // for observer
		Name      string    `json:"name" gomon:"property"`
		Pid       Pid       `json:"pid" gomon:"property"`
		Starttime time.Time `json:"starttime" gomon:"property"`
	}
)

// String formats a pid as a string to comply with fmt.Stringer interface.
func (pid Pid) String() string {
	return strconv.Itoa(int(pid))
}

func (pid Pid) Ancestors(tb Table) []Pid {
	var pids []Pid
	for pid := tb[pid].Ppid; pid > 1; pid = tb[pid].Ppid { // ancestors
		pids = append([]Pid{pid}, pids...)
	}
	return pids
}
