// Copyright © 2021 The Gomon Project.

package process

// lsofCommand starts the lsof command to capture process connections
func lsofCommand(ready chan<- struct{}) {
	ready <- struct{}{}
}
