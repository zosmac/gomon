// Copyright © 2021 The Gomon Project.

package system

import (
	"time"
	"unsafe"

	"github.com/StackExchange/wmi"
	"github.com/zosmac/gomon/core"
	"golang.org/x/sys/windows"
)

var (
	kernel32             = windows.NewLazySystemDLL("kernel32.dll")
	getSystemTimes       = kernel32.NewProc("GetSystemTimes").Call
	globalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx").Call

	pdhapi                      = windows.NewLazySystemDLL("pdh.dll")
	pdhOpenQuery                = pdhapi.NewProc("PdhOpenQueryW").Call
	pdhAddCounter               = pdhapi.NewProc("PdhAddCounterW").Call
	pdhCollectQueryData         = pdhapi.NewProc("PdhCollectQueryData").Call
	pdhGetFormattedCounterValue = pdhapi.NewProc("PdhGetFormattedCounterValue").Call
	pdhRemoveCounter            = pdhapi.NewProc("PdhRemoveCounter").Call
	pdhCloseQuery               = pdhapi.NewProc("PdhCloseQuery").Call
)

// Performance Data Helper
// https://msdn.microsoft.com/en-us/library/windows/desktop/aa373088(v=vs.85).aspx
// To inspect and discover counters, invoke perfmon.exe on Windows.
const (
	// PDH_FMT_* are Windows symbols
	pdhFmtRaw      = 16
	pdhFmtLong     = 256
	pdhFmtDouble   = 512
	pdhFmtLarge    = 1024
	pdhFmtNoScale  = 4096
	pdhFmtNoCap100 = 32768

	contextSwitchesPath = "\\System\\Context Switches/sec"

	// PERF_COUNTER_COUNTER counter type for \System\Context Switches/sec
	perfCounterCounter = 272696320
)

type (
	memoryStatusEx struct {
		dwLength                uint32
		dwMemoryLoad            uint32
		ullTotalPhys            uint64
		ullAvailPhys            uint64
		ullTotalPageFile        uint64
		ullAvailPageFile        uint64
		ullTotalVirtual         uint64
		ullAvailVirtual         uint64
		ullAvailExtendedVirtual uint64
	}

	// Win32_OperatingSystem is a WMI Class for operating system information.
	// The name of a WMI query response object must be identical to the name of a WMI Class,
	// the field names for the query. Go reflection is used to generate the query by the wmi package.
	// See https://docs.microsoft.com/en-us/windows/win32/cimwin32prov/win32-operatingsystem
	win32OperatingSystem struct {
		Name    string
		Version string
	}
)

// uname returns the system name.
func uname() string {
	wos := []win32OperatingSystem{}
	if wmi.Query(wmi.CreateQuery(&wos, ""), &wos) == nil {
		return wos[0].Name + " " + wos[0].Version
	}

	return ""
}

// loadAverage gets the system load averages.
func loadAverage() LoadAverage {
	core.LogInfo(core.Unsupported())
	return LoadAverage{}
}

// contextSwitches queries count of system context switches.
func contextSwitches() int {
	var query uintptr
	if ret, _, _ := pdhOpenQuery(0, 0, uintptr(unsafe.Pointer(&query))); ret != 0 {
		return 0
	}
	defer pdhCloseQuery(query)

	counterPath, _ := windows.UTF16PtrFromString(contextSwitchesPath)
	var counter uintptr
	if ret, _, _ := pdhAddCounter(
		query,
		uintptr(unsafe.Pointer(counterPath)),
		0,
		uintptr(unsafe.Pointer(&counter)),
	); ret != 0 {
		return 0
	}
	defer pdhRemoveCounter(counter)

	if ret, _, _ := pdhCollectQueryData(query); ret != 0 {
		return 0
	}

	<-time.After(1 * time.Second)
	pdhCollectQueryData(query)
	var pdhFmtCountervalue struct {
		status     uint32
		largeValue int
	}
	if ret, _, _ := pdhGetFormattedCounterValue(
		counter,
		pdhFmtLarge+pdhFmtNoScale+pdhFmtNoCap100,
		0,
		uintptr(unsafe.Pointer(&pdhFmtCountervalue)),
	); ret != 0 {
		return 0
	}

	return pdhFmtCountervalue.largeValue
}

func rlimits() Rlimits {
	core.LogInfo(core.Unsupported())
	return Rlimits{}
}

// cpu captures CPU metrics for system.
func cpu() CPU {
	var lpIdleTime, lpKernelTime, lpUserTime windows.Filetime
	_, _, err := getSystemTimes(
		uintptr(unsafe.Pointer(&lpIdleTime)),
		uintptr(unsafe.Pointer(&lpKernelTime)),
		uintptr(unsafe.Pointer(&lpUserTime)),
	)
	if err != nil {
		core.LogError(core.Error("GetSystemTimes", err))
		return CPU{}
	}

	c := CPU{
		User:   time.Duration(lpUserTime.Nanoseconds()),
		System: time.Duration(lpKernelTime.Nanoseconds()),
		Idle:   time.Duration(lpIdleTime.Nanoseconds()),
	}
	c.Total = c.User + c.System + c.Idle
	return c
}

// cpus captures individual CPU metrics.
func cpus() []CPU {
	core.LogInfo(core.Unsupported())
	return nil
}

// memory captures system's memory and swap metrics.
func memory() (Memory, Swap) {
	var memoryStatusEx memoryStatusEx
	_, _, err := globalMemoryStatusEx(
		uintptr(unsafe.Pointer(&memoryStatusEx)),
	)
	if err != nil {
		core.LogError(core.Error("GlobalMemoryStatusEx", err))
	}

	total := memoryStatusEx.ullTotalPhys
	free := memoryStatusEx.ullAvailPhys
	swapTotal := memoryStatusEx.ullTotalPageFile
	swapFree := memoryStatusEx.ullAvailPageFile

	return Memory{
			Total:      int(total),
			Free:       int(free),
			Used:       int(total - free),
			FreeActual: int(free),
			UsedActual: int(total - free),
		},
		Swap{
			Total: int(swapTotal),
			Free:  int(swapFree),
			Used:  int(swapTotal - swapFree),
		}
}
