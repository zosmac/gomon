// Copyright © 2021-2023 The Gomon Project.

package system

/*
#include <stdlib.h>
#include <sys/sysctl.h>
#include <sys/time.h>
#include <mach/mach_host.h>
#include <mach/vm_map.h>
#include <mach/vm_page_size.h>
*/
import "C"

import (
	"strconv"
	"time"
	"unsafe"

	"github.com/zosmac/gocore"
)

type (
	kernReturn C.int
	cpuTicks   [C.CPU_STATE_MAX]C.uint
)

var (
	// uname provides the name of the Operating System.
	uname = func() string {
		var size C.size_t
		if rv, err := C.sysctl(
			&[]C.int{C.CTL_KERN, C.KERN_VERSION}[0],
			2,
			unsafe.Pointer(nil),
			&size,
			unsafe.Pointer(nil),
			0,
		); rv != 0 {
			gocore.Error("sysctl kern.version", err).Err()
			return ""
		}

		version := make([]C.char, size)
		if rv, err := C.sysctl(
			&[]C.int{C.CTL_KERN, C.KERN_VERSION}[0],
			2,
			unsafe.Pointer(&version[0]),
			&size,
			unsafe.Pointer(nil),
			0,
		); rv != 0 {
			gocore.Error("sysctl kern.version", err).Err()
			return ""
		}

		return C.GoString(&version[0])
	}()

	mibNumFiles = func() *C.int {
		name := C.CString("kern.num_files")
		defer C.free(unsafe.Pointer(name))
		mib := make([]C.int, 2)
		count := C.size_t(len(mib))
		C.sysctlnametomib(
			name,
			&mib[0],
			&count,
		)
		return &mib[0]
	}()

	// factor is the system units for CPU time (i.e. "ticks" or "jiffies").
	factor = func() time.Duration {
		var info C.struct_clockinfo
		size := C.size_t(unsafe.Sizeof(info))
		if rv, err := C.sysctl(
			&[]C.int{C.CTL_KERN, C.KERN_CLOCKRATE}[0],
			2,
			unsafe.Pointer(&info),
			&size,
			unsafe.Pointer(nil),
			0,
		); rv != 0 {
			gocore.Error("sysctl kern.clockrate", err).Err()
			return 10000 * time.Microsecond // just guess
		}

		return time.Duration(info.tick) * time.Microsecond
	}()

	// pagesize of memory pages.
	pagesize = int(C.vm_kernel_page_size)
)

func (r kernReturn) Error() string {
	return "mach/kern_return.h: " + strconv.Itoa(int(r))
}

// loadAverage gets the system load averages.
func loadAverage() LoadAverage {
	avg := make([]C.double, 3)

	C.getloadavg(&avg[0], C.int(len(avg)))

	return LoadAverage{
		OneMinute:     float64(avg[0]),
		FiveMinute:    float64(avg[1]),
		FifteenMinute: float64(avg[2]),
	}
}

// contextSwitches queries count of system context switches.
func contextSwitches() int {
	return 0
}

// rlimits gets system resource limits.
func rlimits() Rlimits {
	l := Rlimits{}
	var maxproc, maxprocperuid, maxfiles, maxfilesperproc, numfiles C.int
	var size C.size_t

	size = C.size_t(unsafe.Sizeof(maxproc))
	if rv, err := C.sysctl(
		&[]C.int{C.CTL_KERN, C.KERN_MAXPROC}[0],
		2,
		unsafe.Pointer(&maxproc),
		&size,
		unsafe.Pointer(nil),
		0,
	); rv != 0 {
		gocore.Error("sysctl kern.maxproc", err).Err()
		return l
	}

	size = C.size_t(unsafe.Sizeof(maxfiles))
	if rv, err := C.sysctl(
		&[]C.int{C.CTL_KERN, C.KERN_MAXFILES}[0],
		2,
		unsafe.Pointer(&maxfiles),
		&size,
		unsafe.Pointer(nil),
		0,
	); rv != 0 {
		gocore.Error("sysctl kern.maxfiles", err).Err()
		return l
	}

	size = C.size_t(unsafe.Sizeof(maxfilesperproc))
	if rv, err := C.sysctl(
		&[]C.int{C.CTL_KERN, C.KERN_MAXFILESPERPROC}[0],
		2,
		unsafe.Pointer(&maxfilesperproc),
		&size,
		unsafe.Pointer(nil),
		0,
	); rv != 0 {
		gocore.Error("sysctl kern.maxfilesperproc", err).Err()
		return l
	}

	size = C.size_t(unsafe.Sizeof(maxprocperuid))
	if rv, err := C.sysctl(
		&[]C.int{C.CTL_KERN, C.KERN_MAXPROCPERUID}[0],
		2,
		unsafe.Pointer(&maxprocperuid),
		&size,
		unsafe.Pointer(nil),
		0,
	); rv != 0 {
		gocore.Error("sysctl kern.maxprocperuid", err).Err()
		return l
	}

	size = C.size_t(unsafe.Sizeof(numfiles))
	if rv, err := C.sysctl(
		mibNumFiles,
		2,
		unsafe.Pointer(&numfiles),
		&size,
		unsafe.Pointer(nil),
		0,
	); rv != 0 {
		gocore.Error("sysctl kern.num_files", err).Err()
		return l
	}

	l.ProcessesMaximum = int(maxproc)
	l.ProcessesPerUser = int(maxprocperuid)
	l.OpenFilesMaximum = int(maxfiles)
	l.OpenFilesPerProcess = int(maxfilesperproc)
	l.OpenFilesCurrent = int(numfiles)

	return l
}

// cpu captures CPU metrics for system.
func cpu() CPU {
	var lid C.host_cpu_load_info_data_t
	var number C.mach_msg_type_number_t = C.HOST_CPU_LOAD_INFO_COUNT

	status := C.host_statistics(
		C.host_t(C.mach_host_self()),
		C.HOST_CPU_LOAD_INFO,
		C.host_info_t(unsafe.Pointer(&lid)),
		&number,
	)

	if status != C.KERN_SUCCESS {
		gocore.Error("host_statistics", kernReturn(status)).Err()
		return CPU{}
	}

	return scale(lid.cpu_ticks)
}

// cpus captures individual CPU metrics.
func cpus() []CPU {
	var count C.natural_t
	var info *[1024]C.processor_cpu_load_info_data_t
	var number C.mach_msg_type_number_t

	status := C.host_processor_info(
		C.host_t(C.mach_host_self()),
		C.PROCESSOR_CPU_LOAD_INFO,
		&count,
		(*C.processor_info_array_t)(unsafe.Pointer(&info)),
		&number,
	)

	if status != C.KERN_SUCCESS {
		gocore.Error("host_processor_info", kernReturn(status)).Err()
		return nil
	}

	defer C.vm_deallocate(
		C.vm_map_t(C.mach_host_self()),
		C.vm_address_t(uintptr(unsafe.Pointer(info))),
		C.vm_size_t(number*count),
	)

	cs := make([]CPU, count)
	for i := range cs {
		cs[i] = scale((*info)[i].cpu_ticks)
	}

	return cs
}

// scale converts cpu times to nanoseconds.
func scale(t cpuTicks) CPU {
	c := CPU{
		User:   time.Duration(t[C.CPU_STATE_USER]) * factor,
		System: time.Duration(t[C.CPU_STATE_SYSTEM]) * factor,
		Idle:   time.Duration(t[C.CPU_STATE_IDLE]) * factor,
		Nice:   time.Duration(t[C.CPU_STATE_NICE]) * factor,
	}
	c.Total = c.User + c.System + c.Idle + c.Nice
	return c
}

// memory captures system's memory and swap metrics.
func memory() (Memory, Swap) {
	var memsize C.int64_t
	size := C.size_t(unsafe.Sizeof(memsize))
	if rv, err := C.sysctl(
		&[]C.int{C.CTL_HW, C.HW_MEMSIZE}[0],
		2,
		unsafe.Pointer(&memsize),
		&size,
		unsafe.Pointer(nil),
		0,
	); rv != 0 {
		gocore.Error("sysctl hw.memsize", err).Err()
	}

	var vmi C.struct_vm_statistics64
	var number C.mach_msg_type_number_t = C.HOST_VM_INFO64_COUNT

	status := C.host_statistics64(
		C.host_t(C.mach_host_self()),
		C.HOST_VM_INFO64,
		C.host_info_t(unsafe.Pointer(&vmi)),
		&number,
	)
	if status != C.KERN_SUCCESS {
		gocore.Error("host_statistics", kernReturn(status)).Err()
	}

	total := int(memsize)
	free := int(vmi.free_count) * pagesize
	inactive := int(vmi.inactive_count) * pagesize
	// the following seems to miss some used memory, so use total - free
	// used := int(vmi.active_count+vmi.inactive_count+vmi.wire_count+vmi.compressor_page_count) * pagesize

	var usage C.struct_xsw_usage
	size = C.size_t(unsafe.Sizeof(usage))

	if rv, err := C.sysctl(
		&[]C.int{C.CTL_VM, C.VM_SWAPUSAGE}[0],
		2,
		unsafe.Pointer(&usage),
		&size,
		unsafe.Pointer(nil),
		0,
	); rv != 0 {
		gocore.Error("sysctl vm.swapusage", err).Err()
	}

	return Memory{
			Total:      total,
			Free:       free,
			Used:       total - free,
			FreeActual: free + inactive,
			UsedActual: total - free - inactive,
		},
		Swap{
			Total: int(usage.xsu_total),
			Free:  int(usage.xsu_avail),
			Used:  int(usage.xsu_used),
		}
}
