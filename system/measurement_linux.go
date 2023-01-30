// Copyright © 2021-2023 The Gomon Project.

package system

// Rlimits of system
type Rlimits struct {
	MemoryPerUser       int `json:"memory_per_user" gomon:"gauge,B"`
	ProcessesPerUser    int `json:"processes_per_user" gomon:"gauge,count"`
	OpenFilesMaximum    int `json:"open_files_maximum" gomon:"gauge,count"`
	OpenFilesPerProcess int `json:"open_files_per_process" gomon:"gauge,count"`
}
