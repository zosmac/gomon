// Copyright © 2021 The Gomon Project.

package process

// getInodes is called from Measure(), but only relevant for Linux, so for Windows is a noop.
func getInodes() {}

// endpoints returns a list of Connection structures for a process.
func (pid Pid) endpoints() Connections {
	return Connections{}
}
