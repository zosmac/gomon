// Copyright © 2021-2023 The Gomon Project.

package process

import (
	"time"
	"unsafe"

	"github.com/zosmac/gocore"

	"github.com/yusufpapurcu/wmi"

	"golang.org/x/sys/windows"
)

type (
	// PROCESS_MEMORY_COUNTERS_EX contains process memory counters.
	processMemoryCountersEx struct {
		cb                         uint32
		PageFaultCount             uint32
		PeakWorkingSetSize         uintptr
		WorkingSetSize             uintptr
		QuotaPeakPagedPoolUsage    uintptr
		QuotaPagedPoolUsage        uintptr
		QuotaPeakNonPagedPoolUsage uintptr
		QuotaNonPagedPoolUsage     uintptr
		PagefileUsage              uintptr
		PeakPagefileUsage          uintptr
		PrivateUsage               uintptr
	}

	// PROCESS_IO_COUNTERS contains process I/O counters
	processIOCounters struct {
		ReadOperationCount  uint64
		WriteOperationCount uint64
		OtherOperationCount uint64
		ReadTransferCount   uint64
		WriteTransferCount  uint64
		OtherTransferCount  uint64
	}

	// Win32_Process is a WMI Class for process information.
	// The name of a WMI query response object must be identical to the name of a WMI Class, as do
	// the field names for the query. Go reflection is used to generate the query by the wmi package.
	// See https://docs.microsoft.com/en-us/windows/win32/cimwin32prov/win32-process
	win32Process struct {
		CommandLine         string
		CreationDate        time.Time // GetProcessTimes also gets this
		ExecutablePath      string    // GetProcessImageFileName also gets this
		ExecutionState      uint16
		KernelModeTime      uint64 // GetProcessTimes also gets this
		Name                string
		PageFaults          uint32 // GetProcessMemoryInfo also gets this
		ParentProcessID     uint32
		ProcessID           uint32
		ReadOperationCount  uint64 // GetProcessIoCounters also gets this
		ReadTransferCount   uint64 // GetProcessIoCounters also gets this
		ThreadCount         uint32
		UserModeTime        uint64 // GetProcessTimes also gets this
		VirtualSize         uint64 // GetProcessMemoryInfo also gets this
		WorkingSetSize      uint64 // GetProcessMemoryInfo also gets this
		WriteOperationCount uint64 // GetProcessIoCounters also gets this
		WriteTransferCount  uint64 // GetProcessIoCounters also gets this
	}
)

var (
	kernel32             = windows.NewLazySystemDLL("kernel32.dll")
	getProcessIoCounters = kernel32.NewProc("GetProcessIoCounters").Call

	psapi                   = windows.NewLazySystemDLL("psapi.dll")
	getProcessImageFileName = psapi.NewProc("GetProcessImageFileNameW").Call
	getProcessMemoryInfo    = psapi.NewProc("GetProcessMemoryInfo").Call

	status = map[uint16]string{ // Win32_Process ExecutionState
		0: "Unknown",
		1: "Other",
		2: "Ready",
		3: "Running",
		4: "Blocked",
		5: "Suspended Blocked",
		6: "Suspended Ready",
		7: "Terminated",
		8: "Stopped",
		9: "Growing",
	}
)

const (
	processAllAccess = 0x001f0fff
	stillActive      = 259
)

// metrics captures the metrics for a process.
func (pid Pid) metrics() (Id, Properties, Metrics) {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_INFORMATION|windows.PROCESS_VM_READ, false, uint32(pid))
	if err != nil {
		gocore.Error("OpenProcess", err).Err()
		return Id{Pid: pid}, Properties{}, Metrics{}
	}
	defer windows.CloseHandle(handle)

	var token windows.Token
	if err = windows.OpenProcessToken(handle, windows.TOKEN_QUERY, &token); err != nil {
		gocore.Error("OpenProcessToken", err).Info()
	}

	u, err := token.GetTokenUser()
	if err != nil {
		gocore.Error("GetTokenUser", err).Info()
	}

	account, domain, _, err := u.User.Sid.LookupAccount("")
	if err != nil {
		gocore.Error("LookupAccount", err).Info()
	}

	var name [windows.MAX_PATH + 1]uint16
	n, _, err := getProcessImageFileName(
		uintptr(handle),
		uintptr(unsafe.Pointer(&name[0])),
		windows.MAX_PATH+1,
	)
	if n == 0 {
		gocore.Error("GetProcessImageFileName", err).Info()
	}

	var lpCreationTime, lpExitTime, lpKernelTime, lpUserTime windows.Filetime
	err = windows.GetProcessTimes(
		handle,
		&lpCreationTime,
		&lpExitTime,
		&lpKernelTime,
		&lpUserTime,
	)
	if err != nil {
		gocore.Error("GetProcessTimes", err).Info()
	}

	processMemoryCounters := processMemoryCountersEx{
		cb: uint32(unsafe.Sizeof(processMemoryCountersEx{})),
	}
	_, _, err = getProcessMemoryInfo(
		uintptr(handle),
		uintptr(unsafe.Pointer(&processMemoryCounters)),
		uintptr(unsafe.Sizeof(processMemoryCounters)),
	)
	if err != nil {
		gocore.Error("GetProcessMemoryInfo", err).Info()
	}

	wp := []win32Process{}
	if err = wmi.Query(
		wmi.CreateQuery(
			&wp,
			"WHERE ProcessId = "+pid.String(),
		),
		&wp,
	); err != nil {
		gocore.Error("Win32_Process", err).Info()
		wp = []win32Process{win32Process{}}
	}

	// The Win32_Process class has a GetOwner method that the process user and domain, but how does one call it?

	user := time.Duration(lpUserTime.Nanoseconds())
	system := time.Duration(lpKernelTime.Nanoseconds())

	return Id{
			Name:      windows.UTF16ToString(name[:n]),
			Pid:       Pid(wp[0].ProcessID),
			Starttime: time.Unix(0, lpCreationTime.Nanoseconds()),
		},
		Properties{
			Ppid:        Pid(wp[0].ParentProcessID),
			Username:    account + "@" + domain,
			Status:      status[wp[0].ExecutionState],
			CommandLine: pid.commandLine(),
			Directories: pid.directories(),
		},
		Metrics{
			Threads:    int(wp[0].ThreadCount),
			User:       user,
			System:     system,
			Total:      user + system,
			Size:       int(processMemoryCounters.PrivateUsage),
			Resident:   int(processMemoryCounters.WorkingSetSize),
			PageFaults: int(processMemoryCounters.PageFaultCount),
			Io:         pid.io(),
		}
}

// io captures process I/O counts.
func (pid Pid) io() Io {
	handle, err := windows.OpenProcess(processAllAccess, false, uint32(pid))
	if err != nil {
		gocore.Error("OpenProcess", err).Err()
		return Io{}
	}
	defer windows.CloseHandle(handle)

	var ioCounters processIOCounters
	ret, _, err := getProcessIoCounters(
		uintptr(handle),
		uintptr(unsafe.Pointer(&ioCounters)),
	)
	if ret == 0 {
		gocore.Error("GetProcessIoCounters", err).Err()
		return Io{}
	}

	return Io{
		ReadActual:      int(ioCounters.ReadTransferCount),
		WriteActual:     int(ioCounters.WriteTransferCount),
		ReadOperations:  int(ioCounters.ReadOperationCount),
		WriteOperations: int(ioCounters.WriteOperationCount),
	}
}

// commandLine retrieves process command, arguments, and environment.
func (pid Pid) commandLine() CommandLine {
	// this could be populated with the results of the Win32_Process CommandLine field
	return CommandLine{}
}

// directories captures process directories.
func (pid Pid) directories() Directories {
	return Directories{}
}

// getPids gets the list of active processes by pid.
func getPids() ([]Pid, error) {
	bytes := uint32(4096 * 4)
	var ps []uint32
	for {
		bytes *= 2
		ps = make([]uint32, bytes/4)
		if err := windows.EnumProcesses(ps, &bytes); err != nil {
			return nil, gocore.Error("EnumProcesses", err)
		}
		if int(bytes) < len(ps)*4 {
			break
		}
	}
	ps = ps[:bytes/4]
	pids := make([]Pid, 0, len(ps))
	for i, pid := range ps {
		pids[i] = Pid(pid)
	}
	return pids, nil
}
