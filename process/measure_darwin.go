// Copyright © 2021-2023 The Gomon Project.

package process

/*
#include <libproc.h>
#include <sys/sysctl.h>
*/
import "C"

import (
	"bytes"
	"fmt"
	"time"
	"unsafe"

	"github.com/zosmac/gocore"
)

var (
	status = map[C.uint]string{
		C.SIDL:   "Idle",
		C.SRUN:   "Running",
		C.SSLEEP: "Sleeping",
		C.SSTOP:  "Stopped",
		C.SZOMB:  "Zombie",
	}
)

// id gets the identifier of a process.
func (pid Pid) id() (Id, error) {
	var bsd C.struct_proc_bsdinfo
	if n, err := C.proc_pidinfo(
		C.int(pid),
		C.PROC_PIDTBSDINFO,
		0,
		unsafe.Pointer(&bsd),
		C.int(C.PROC_PIDTBSDINFO_SIZE),
	); n != C.int(C.PROC_PIDTBSDINFO_SIZE) {
		return Id{Pid: pid}, gocore.Error("proc_pidinfo PROC_PIDTBSDINFO failed", err)
	}

	name := C.GoString(&bsd.pbi_name[0])
	if name == "" {
		name = C.GoString(&bsd.pbi_comm[0])
	}

	return Id{
		ppid: Pid(bsd.pbi_ppid),
		Name: name,
		Pid:  pid,
		Starttime: time.Unix(
			int64(bsd.pbi_start_tvsec),
			int64(bsd.pbi_start_tvusec)*int64(time.Microsecond),
		),
	}, nil
}

// metrics captures the metrics for a process.
func (pid Pid) metrics() (Id, Properties, Metrics) {
	var tai C.struct_proc_taskallinfo
	if n := C.proc_pidinfo(
		C.int(pid),
		C.PROC_PIDTASKALLINFO,
		0,
		unsafe.Pointer(&tai),
		C.int(C.PROC_PIDTASKALLINFO_SIZE),
	); n != C.int(C.PROC_PIDTASKALLINFO_SIZE) {
		return Id{Pid: pid}, Properties{}, Metrics{}
	}

	name := C.GoString(&tai.pbsd.pbi_name[0])
	if name == "" {
		name = C.GoString(&tai.pbsd.pbi_comm[0])
	}
	user, system := pid.threads(tai.ptinfo.pti_threadnum)
	user += time.Duration(tai.ptinfo.pti_total_user + tai.ptinfo.pti_threads_user)
	system += time.Duration(tai.ptinfo.pti_total_system + tai.ptinfo.pti_threads_system)

	return Id{
			ppid: Pid(tai.pbsd.pbi_ppid),
			Name: name,
			Pid:  pid,
			Starttime: time.Unix(
				int64(tai.pbsd.pbi_start_tvsec),
				int64(tai.pbsd.pbi_start_tvusec)*int64(time.Microsecond),
			),
		},
		Properties{
			Ppid:        Pid(tai.pbsd.pbi_ppid),
			Pgid:        int(tai.pbsd.pbi_pgid),
			Tty:         fmt.Sprintf("%#.8X", tai.pbsd.e_tdev),
			UID:         int(tai.pbsd.pbi_uid),
			GID:         int(tai.pbsd.pbi_gid),
			Username:    gocore.Username(int(tai.pbsd.pbi_uid)),
			Groupname:   gocore.Groupname(int(tai.pbsd.pbi_gid)),
			Status:      status[tai.pbsd.pbi_status],
			Nice:        int(tai.pbsd.pbi_nice),
			CommandLine: pid.commandLine(),
			Directories: pid.directories(),
		},
		Metrics{
			Priority:        int(tai.ptinfo.pti_priority),
			Threads:         int(tai.ptinfo.pti_threadnum),
			User:            user,
			System:          system,
			Total:           user + system,
			Size:            int(tai.ptinfo.pti_virtual_size),
			Resident:        int(tai.ptinfo.pti_resident_size),
			PageFaults:      int(tai.ptinfo.pti_faults),
			ContextSwitches: int(tai.ptinfo.pti_csw),
			Io:              pid.io(),
		}
}

// threads captures cpu time for each thread of a process.
func (pid Pid) threads(num C.int) (time.Duration, time.Duration) {
	var tid C.uint64_t
	tids := make([]C.uint64_t, num+10) // include some padding
	n := C.proc_pidinfo(
		C.int(pid),
		C.PROC_PIDLISTTHREADS,
		0,
		unsafe.Pointer(&tids[0]),
		(num+10)*C.int(unsafe.Sizeof(tid)),
	)
	if n <= 0 {
		return 0, 0
	}
	n /= C.int(unsafe.Sizeof(tid))
	if n < num+10 {
		tids = tids[:n]
	}

	var user, system time.Duration
	for _, tid := range tids {
		var pti C.struct_proc_threadinfo
		if n := C.proc_pidinfo(
			C.int(pid),
			C.PROC_PIDTHREADINFO,
			tid,
			unsafe.Pointer(&pti),
			C.int(C.PROC_PIDTHREADINFO_SIZE),
		); n == C.PROC_PIDTHREADINFO_SIZE && (pti.pth_flags&C.TH_FLAGS_IDLE == 0) { // idea from libtop.c
			user += time.Duration(pti.pth_user_time)
			system += time.Duration(pti.pth_system_time)
		}
	}

	return user, system
}

// io captures process I/O counts.
func (pid Pid) io() Io {
	var ric C.rusage_info_current
	if rv := C.proc_pid_rusage(
		C.int(pid),
		C.RUSAGE_INFO_CURRENT,
		(*C.rusage_info_t)(unsafe.Pointer(&ric)),
	); rv != 0 {
		return Io{}
	}

	return Io{
		ReadActual:     int(ric.ri_diskio_bytesread),
		WriteActual:    int(ric.ri_diskio_byteswritten),
		WriteRequested: int(ric.ri_logical_writes),
	}
}

// commandLine retrieves process command, arguments, and environment.
func (pid Pid) commandLine() CommandLine {
	clLock.RLock()
	cl, ok := clMap[pid]
	clLock.RUnlock()
	if ok {
		return cl
	}

	size := C.size_t(C.ARG_MAX)
	buf := make([]byte, size)

	if rv := C.sysctl(
		(*C.int)(unsafe.Pointer(&[3]C.int{C.CTL_KERN, C.KERN_PROCARGS2, C.int(pid)})),
		3,
		unsafe.Pointer(&buf[0]),
		&size,
		unsafe.Pointer(nil),
		0,
	); rv != 0 {
		return CommandLine{}
	}

	l := int(*(*uint32)(unsafe.Pointer(&buf[0])))
	ss := bytes.FieldsFunc(buf[4:size], func(r rune) bool { return r == 0 })
	var executable string
	var args, envs []string
	for i, s := range ss {
		if i == 0 {
			executable = string(s)
		} else if i <= l {
			args = append(args, string(s))
		} else {
			envs = append(envs, string(s))
		}
	}

	cl = CommandLine{
		Executable: executable,
		Args:       args,
		Envs:       envs,
	}
	clLock.Lock()
	clMap[pid] = cl
	clLock.Unlock()
	return cl
}

// directories captures process directories.
func (pid Pid) directories() Directories {
	var vpi C.struct_proc_vnodepathinfo
	if n := C.proc_pidinfo(
		C.int(pid),
		C.PROC_PIDVNODEPATHINFO,
		0,
		unsafe.Pointer(&vpi),
		C.int(C.PROC_PIDVNODEPATHINFO_SIZE),
	); n != C.int(C.PROC_PIDVNODEPATHINFO_SIZE) {
		return Directories{}
	}

	return Directories{
		Cwd:  C.GoString(&vpi.pvi_cdir.vip_path[0]),
		Root: C.GoString(&vpi.pvi_rdir.vip_path[0]),
	}
}

// getPids gets the list of active processes by pid.
func getPids() ([]Pid, error) {
	n, err := C.proc_listpids(C.PROC_ALL_PIDS, 0, nil, 0)
	if n <= 0 {
		return nil, gocore.Error("proc_listpids PROC_ALL_PIDS failed", err)
	}

	var pid C.int
	buf := make([]C.int, n/C.int(unsafe.Sizeof(pid))+10)
	if n, err = C.proc_listpids(C.PROC_ALL_PIDS, 0, unsafe.Pointer(&buf[0]), n); n <= 0 {
		return nil, gocore.Error("proc_listpids PROC_ALL_PIDS failed", err)
	}
	n /= C.int(unsafe.Sizeof(pid))
	if int(n) < len(buf) {
		buf = buf[:n]
	}

	pids := make([]Pid, len(buf))
	for i, pid := range buf {
		pids[int(n)-i-1] = Pid(pid) // Darwin returns pids in descending order, so reverse the order
	}
	return pids, nil
}
