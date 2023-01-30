// Copyright © 2021-2023 The Gomon Project.

package system

// Rlimits of system
type Rlimits struct {
	ProcessesMaximum    int `json:"processes_maximum" gomon:"gauge,count"`
	ProcessesPerUser    int `json:"processes_per_user" gomon:"gauge,count"`
	OpenFilesMaximum    int `json:"open_files_maximum" gomon:"gauge,count"`
	OpenFilesPerProcess int `json:"open_files_per_process" gomon:"gauge,count"`
	OpenFilesCurrent    int `json:"open_files_current" gomon:"gauge,count"`
}
