// Copyright © 2021-2023 The Gomon Project.

package process

/*
#include <libproc.h>
#include <sys/sysctl.h>
*/
import "C"

import (
	"errors"
	"syscall"
	"unsafe"

	"github.com/zosmac/gocore"
)

type (
	// ids maps pids to current process instances.
	ids map[Pid]Id
)

var (
	// families identifies parent-child relationships.
	families = map[Pid]ids{}
)

func open() error {
	return nil
}

// watch adds a process to watch to the observer.
func watch(kd int, pid Pid) error {
	if _, err := syscall.Kevent(
		kd,
		[]syscall.Kevent_t{{
			Ident:  uint64(pid),
			Filter: syscall.EVFILT_PROC,
			Flags:  syscall.EV_ADD | syscall.EV_CLEAR,                         // | syscall.EV_RECEIPT,
			Fflags: syscall.NOTE_FORK | syscall.NOTE_EXEC | syscall.NOTE_EXIT, // | syscall.NOTE_EXITSTATUS | syscall.NOTE_EXIT_DETAIL | syscall.NOTE_SIGNAL,
		}},
		nil,
		nil,
	); err != nil {
		return gocore.Error("Kevent", err)
	}

	return nil
}

// observe events and notify observer's callbacks.
func observe() error {
	pids, err := getPids()
	if err != nil {
		return gocore.Error("getPids", err)
	}

	kd, err := syscall.Kqueue()
	if err != nil {
		return gocore.Error("Kqueue", err)
	}

	go func() {
		defer syscall.Close(kd)

		for _, pid := range pids {
			families[pid] = children(pid)
		}
		for ppid, pids := range families {
			for pid := range pids {
				id, err := pid.id()
				if err != nil {
					gocore.Error("fork", err, map[string]string{
						"ppid": ppid.String(),
						"pid":  pid.String(),
					}).Err()
					continue
				}
				if err := watch(kd, pid); err != nil {
					gocore.Error("fork", err, map[string]string{
						"ppid": ppid.String(),
						"pid":  pid.String(),
					}).Err()
					continue
				}
				id.ppid = ppid // preserve in case child reassigned to init process
				families[ppid][pid] = id
			}
		}
		for {
			events := make([]syscall.Kevent_t, 10)
			n, err := syscall.Kevent(kd, nil, events, nil)
			if err != nil {
				if errors.Is(err, syscall.EINTR) {
					continue
				}
				gocore.Error("Kevent", err).Err()
				return
			}

			for _, event := range events[:n] {
				pid := Pid(event.Ident)
				if event.Flags&syscall.EV_ERROR != 0 {
					err := syscall.Errno(event.Data)
					gocore.Error("Kevent", err, map[string]string{
						"pid": pid.String(),
					}).Err()
					continue
				}

				if event.Fflags&syscall.NOTE_FORK != 0 {
					newKids(kd, pid)
					continue
				}

				var id Id
				var ok bool
				var ppid Pid
				var youth ids
				for ppid, youth = range families {
					if id, ok = youth[pid]; ok {
						break
					}
				}

				if event.Fflags&syscall.NOTE_EXEC != 0 {
					if id, err = pid.id(); err != nil {
						gocore.Error("exec", err, map[string]string{
							"pid": pid.String(),
						}).Err()
						if ok {
							delete(youth, pid)
						}
						continue
					}
					if ok {
						id.ppid = ppid
					}
					families[id.ppid][pid] = id
					id.exec()
				}
				if event.Fflags&syscall.NOTE_EXIT != 0 { // | syscall.NOTE_EXITSTATUS | syscall.NOTE_EXIT_DETAIL:
					delete(families[id.ppid], pid)
					delete(families, pid)
					id.Pid = pid
					id.exit()
				}
			}
		}
	}()

	return nil
}

// newKids identifies new kids of process and recreates existing kids list.
func newKids(kd int, ppid Pid) {
	family := families[ppid]
	families[ppid] = children(ppid)
	for pid := range families[ppid] {
		if id, ok := family[pid]; ok {
			families[ppid][pid] = id
			continue
		}
		id, err := pid.id()
		if err == nil {
			err = watch(kd, pid)
		}
		if err != nil {
			if !errors.Is(err, syscall.ESRCH) {
				gocore.Error("fork", err, map[string]string{
					"ppid": ppid.String(),
					"pid":  pid.String(),
				}).Err()
			}
			continue
		}
		id.ppid = ppid // preserve in case child reassigned to init process
		families[ppid][pid] = id
		id.fork()
		newKids(kd, pid) // continue with descendants
	}
}

// children identifies existing kids of parent.
func children(ppid Pid) ids {
	n := C.proc_listpids(C.PROC_PPID_ONLY, C.uint32_t(ppid), nil, 0)
	if n <= 0 {
		return nil
	}

	pids := make([]C.int, n/C.sizeof_pid_t)
	if n = C.proc_listpids(C.PROC_PPID_ONLY, C.uint32_t(ppid), unsafe.Pointer(&pids[0]), n); n <= 0 {
		return nil
	}
	n /= C.sizeof_pid_t

	kids := ids{}
	for _, pid := range pids[:n] {
		kids[Pid(pid)] = Id{}
	}

	return kids
}
