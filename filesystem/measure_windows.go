// Copyright © 2021-2023 The Gomon Project.

package filesystem

import (
	"github.com/zosmac/gocore"
	"github.com/zosmac/gomon/message"
	"golang.org/x/sys/windows"
)

// filesystems returns a list of filesystems.
func filesystems() ([]message.Request, error) {
	var driveStrings [4*26 + 1]uint16
	var path [windows.MAX_PATH + 1]uint16
	var fsType [windows.MAX_PATH + 1]uint16
	var target [windows.MAX_PATH + 1]uint16

	n, err := windows.GetLogicalDriveStrings(
		uint32(len(driveStrings)-1),
		&driveStrings[0],
	)
	if n == 0 {
		return nil, gocore.Error("GetLogicalDriveStrings", err)
	}

	var qs []message.Request
	var drive string
	for i := 0; i < int(n); i += len(drive) + 1 {
		drive = windows.UTF16ToString(driveStrings[i:])
		if len(drive) == 0 {
			break // The drive strings list ends with 2 null terminators
		}
		qs = append(qs,
			func() []message.Content {
				driveType := windows.GetDriveType(&driveStrings[i])
				path[0], fsType[0], target[0] = 0, 0, 0
				windows.GetVolumeInformation(
					&driveStrings[i],
					&path[0],
					uint32(len(path)-1),
					nil,
					nil,
					nil,
					&fsType[0],
					uint32(len(fsType)-1),
				)
				driveStrings[i+len(drive)-1] = 0 // remove trailing "\"
				l, _ := windows.QueryDosDevice(
					&driveStrings[i],
					&target[0],
					uint32(len(target)-1),
				)
				return []message.Content{
					&measurement{
						Header: message.Measurement(),
						Id: Id{
							Mount: drive,
							Path:  windows.UTF16ToString(path[:]),
						},
						Properties: Properties{
							Type:      windows.UTF16ToString(fsType[:]),
							DriveType: gocore.DriveTypes[driveType],
							Device:    windows.UTF16ToString(target[:l]),
						},
					},
				}
			},
		)
	}
	return qs, nil
}
