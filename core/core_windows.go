// Copyright © 2021 The Gomon Project.

package core

import (
	"errors"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"time"
	"unsafe"

	"github.com/StackExchange/wmi"

	"golang.org/x/sys/windows"
)

var (
	kernel32                 = windows.NewLazySystemDLL("kernel32.dll")
	getFinalPathNameByHandle = kernel32.NewProc("GetFinalPathNameByHandleW").Call

	// DriveTypes maps DRIVE keys to names.
	DriveTypes = map[uint32]string{
		windows.DRIVE_UNKNOWN:     "unknown",
		windows.DRIVE_NO_ROOT_DIR: "no_root_dir",
		windows.DRIVE_REMOVABLE:   "removable",
		windows.DRIVE_FIXED:       "fixed",
		windows.DRIVE_REMOTE:      "remote",
		windows.DRIVE_CDROM:       "cdrom",
		windows.DRIVE_RAMDISK:     "ramdisk",
	}
)

const (
	volumeNameDOS = 0
	volumeNameNT  = 2
)

// signalChannel returns channel on which OS signals are delivered.
func signalChannel() <-chan os.Signal {
	signalChan := make(chan os.Signal, 1)   // use buffered channel to ensure signal delivery
	signal.Notify(signalChan, os.Interrupt) // only os.Interrupt (ctrl-C, ctrl-Break) can be caught on Windows
	return signalChan
}

// setuid not implemented in Windows.
func setuid() {
}

// FdPath gets the path for an open file descriptor
func FdPath(fd int) (string, error) {
	var wchar [windows.MAX_PATH + 1]uint16
	n, _, err := getFinalPathNameByHandle(
		uintptr(fd),
		uintptr(unsafe.Pointer(&wchar[0])),
		windows.MAX_PATH+1,
		volumeNameDOS,
	)
	if n == 0 {
		return "", NewError("GetFinalPathNameByHandle", err)
	}

	path := windows.UTF16ToString(wchar[:n])
	strings.TrimPrefix(path, `\\?\`)
	return path, nil
}

// MountMap builds a map of mount points to file systems.
func MountMap() (map[string]string, error) {
	return map[string]string{}, NewError(runtime.GOOS, errors.New("unsupported"))
}

// Win32_OperatingSystem is a WMI Class for operating system information.
// The name of a WMI query response object must be identical to the name of a WMI Class,
// the field names for the query. Go reflection is used to generate the query by the wmi package.
// See https://docs.microsoft.com/en-us/windows/win32/cimwin32prov/win32-operatingsystem
type win32OperatingSystem struct {
	LastBootUpTime time.Time // Field names in the structure must match names in the WMI Class
}

// boottime gets the system boot time.
func boottime() time.Time {
	wos := []win32OperatingSystem{}
	if wmi.Query(wmi.CreateQuery(&wos, ""), &wos) == nil {
		return wos[0].LastBootUpTime
	}
	return time.Time{}
}
