// Copyright © 2021 The Gomon Project.

//go:build linux
// +build linux

package process

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"syscall"
	"unsafe"

	"github.com/zosmac/gomon/core"
	"golang.org/x/net/bpf"
	"golang.org/x/sys/unix"
)

/*
Linux Netlink API and BPF Reference:
	http://man7.org/linux/man-pages/man7/netlink.7.html
	http://man7.org/linux/man-pages/man2/bpf.2.html
	https://godoc.org/golang.org/x/net/bpf

netlink process connector
	/usr/include/linux/netlink.h
	/usr/include/linux/connector.h
	/usr/include/linux/cn_proc.h

netlink generic connector
	/usr/include/linux/netlink.h
	/usr/include/net/linux/genetlink.h
	/usr/include/uapi/linux/genetlink.h
	/usr/include/uapi/linux/netlink.h
	/usr/include/uapi/linux/taskstats.h
*/

// nlmsgAlign from /usr/include/linux/netlink.h
func nlmsgAlign(l int) uintptr {
	return uintptr((l + syscall.NLMSG_ALIGNTO - 1) & ^(syscall.NLMSG_ALIGNTO - 1))
}

// *************************************************
// NETLINK process connector convenience definitions
// *************************************************
type (
	// proc_cn_mcast_op enum from /usr/include/linux/cn_proc.h
	procCnMcastOp uint32

	// proc_event.what enum from /usr/include/linux/cn_proc.h
	what uint32
)

const (
	// netlink connector routing ids from linux/connector.h
	cnIdxProc = 1
	cnValProc = 1

	connectorMaxMessageSize = 16384

	// enum procCnMcastOp
	procCnMcastListen = procCnMcastOp(1)
	procCnMcastIgnore = procCnMcastOp(2)

	// enum proc_event.what
	procEventFork = what(0x00000001)
	procEventExec = what(0x00000002)
	procEventUID  = what(0x00000004)
	procEventGID  = what(0x00000040)
	procEventSID  = what(0x00000080)
	procEventComm = what(0x00000200)
	procEventExit = what(0x80000000)
)

type (
	// nlProcRequest defines netlink process request.
	nlProcRequest struct {
		syscall.NlMsghdr
		cnMsg
		procCnMcastOp
	}

	// nlProcResponse defines netlink process response header.
	nlProcResponse struct {
		syscall.NlMsghdr
		cnMsg
		procEvent
	}

	// struct cb_id from /usr/include/linux/connector.h
	cbID struct {
		idx uint32
		val uint32
	}

	// struct cn_msg from /usr/include/linux/connector.h
	cnMsg struct {
		cbID
		seq   uint32
		ack   uint32
		len   uint16
		flags uint16
	}

	// struct proc_event from /usr/include/linux/uapi/cn_proc.h
	procEvent struct {
		what
		cpu       uint32
		timestamp [2]uint32 // nanoseconds since system boot, specify as [2]uint32 (rather than uint64) to prevent doubleword alignment of struct
	}

	// struct fork_proc_event from /usr/include/uapi/linux/cn_proc.h
	forkProcEvent struct {
		parentPid  uint32
		parentTgid uint32
		childPid   uint32
		childTgid  uint32
	}

	// struct exec_proc_event from /usr/include/uapi/linux/cn_proc.h
	execProcEvent struct {
		processPid  uint32
		processTgid uint32
	}

	// struct exit_proc_event from /usr/include/uapi/linux/cn_proc.h
	exitProcEvent struct {
		processPid  uint32
		processTgid uint32
		exitCode    uint32
		exitSignal  uint32
	}

	// struct uid_proc_event from /usr/include/uapi/linux/cn_proc.h
	uidProcEvent struct {
		processPid  uint32
		processTgid uint32
		uid         uint32
		euid        uint32
	}

	// struct gid_proc_event from /usr/include/uapi/linux/cn_proc.h
	gidProcEvent struct {
		processPid  uint32
		processTgid uint32
		gid         uint32
		egid        uint32
	}

	// struct sid_proc_event from /usr/include/uapi/linux/cn_proc.h
	sidProcEvent struct {
		processPid  uint32
		processTgid uint32
	}

	// struct comm_proc_event from /usr/include/uapi/linux/cn_proc.h
	commProcEvent struct {
		processPid  uint32
		processTgid uint32
		comm        [16]byte
	}
)

// nlProcess opens socket to the netlink process connector.
func nlProcess() (int, error) {
	fd, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_DGRAM, syscall.NETLINK_CONNECTOR)
	if err != nil {
		return -1, core.NewError("socket", err)
	}

	if err := syscall.Bind(fd,
		&syscall.SockaddrNetlink{
			Family: syscall.AF_NETLINK,
			Groups: cnIdxProc,
		},
	); err != nil {
		syscall.Close(fd)
		return -1, core.NewError("bind", err)
	}

	if err := nlProcessFilter(fd); err != nil {
		syscall.Close(fd)
		return -1, core.NewError("setsockopt attach filter", err)
	}

	if err := nlProcessListen(fd, true); err != nil {
		syscall.Close(fd)
		return -1, core.NewError("netlink process connector listen", err)
	}

	return fd, nil
}

// nlProcessFilter sets filter program for netlink process connector socket.
// Inspiration: http://netsplit.com/the-proc-connector-and-socket-filters
// godoc for bpf package: https://godoc.org/golang.org/x/net/bpf
func nlProcessFilter(fd int) error {
	filter, err := bpf.Assemble([]bpf.Instruction{
		bpf.LoadAbsolute{Off: uint32(nlmsgAlign(0) + unsafe.Offsetof(nlProcResponse{}.Type)), Size: 2},
		bpf.JumpIf{Cond: bpf.JumpEqual, Val: uint32(core.Htons(syscall.NLMSG_DONE)), SkipTrue: 1}, // NLMSG_DONE
		bpf.RetConstant{Val: 0}, // filter out
		bpf.LoadAbsolute{Off: uint32(nlmsgAlign(0) + unsafe.Offsetof(nlProcResponse{}.idx)), Size: 4},
		bpf.JumpIf{Cond: bpf.JumpEqual, Val: core.Htonl(cnIdxProc), SkipTrue: 1}, // CN_IDX_PROC
		bpf.RetConstant{Val: 0}, // filter out
		bpf.LoadAbsolute{Off: uint32(nlmsgAlign(0) + unsafe.Offsetof(nlProcResponse{}.val)), Size: 4},
		bpf.JumpIf{Cond: bpf.JumpEqual, Val: core.Htonl(cnValProc), SkipTrue: 1}, // CN_VAL_PROC
		bpf.RetConstant{Val: 0}, // filter out
		bpf.LoadAbsolute{Off: uint32(nlmsgAlign(0) + unsafe.Offsetof(nlProcResponse{}.what)), Size: 4},
		// FORK filter
		bpf.JumpIf{Cond: bpf.JumpEqual, Val: core.Htonl(uint32(procEventFork)), SkipFalse: 8},
		bpf.LoadAbsolute{Off: uint32(nlmsgAlign(0) + unsafe.Sizeof(nlProcResponse{}) + unsafe.Offsetof(forkProcEvent{}.childPid)), Size: 4},
		bpf.StoreScratch{Src: bpf.RegA, N: 0},
		bpf.LoadScratch{Dst: bpf.RegX, N: 0},
		bpf.LoadAbsolute{Off: uint32(nlmsgAlign(0) + unsafe.Sizeof(nlProcResponse{}) + unsafe.Offsetof(forkProcEvent{}.childTgid)), Size: 4},
		bpf.JumpIfX{Cond: bpf.JumpEqual, SkipTrue: 1}, // child_pid == child_tgid (i.e. this is a fork(), not a clone()
		bpf.RetConstant{Val: 0},                       // filter out
		bpf.RetConstant{Val: 0xFFFFFFFF},
		// EXEC filter
		bpf.JumpIf{Cond: bpf.JumpEqual, Val: core.Htonl(uint32(procEventExec)), SkipFalse: 8},
		bpf.LoadAbsolute{Off: uint32(nlmsgAlign(0) + unsafe.Sizeof(nlProcResponse{}) + unsafe.Offsetof(execProcEvent{}.processPid)), Size: 4},
		bpf.StoreScratch{Src: bpf.RegA, N: 0},
		bpf.LoadScratch{Dst: bpf.RegX, N: 0},
		bpf.LoadAbsolute{Off: uint32(nlmsgAlign(0) + unsafe.Sizeof(nlProcResponse{}) + unsafe.Offsetof(execProcEvent{}.processTgid)), Size: 4},
		bpf.JumpIfX{Cond: bpf.JumpEqual, SkipTrue: 1}, // exec process_pid == process_tgid
		bpf.RetConstant{Val: 0},                       // filter out
		bpf.RetConstant{Val: 0xFFFFFFFF},
		// EXIT filter
		bpf.JumpIf{Cond: bpf.JumpEqual, Val: core.Htonl(uint32(procEventExit)), SkipFalse: 8},
		bpf.LoadAbsolute{Off: uint32(nlmsgAlign(0) + unsafe.Sizeof(nlProcResponse{}) + unsafe.Offsetof(exitProcEvent{}.processPid)), Size: 4},
		bpf.StoreScratch{Src: bpf.RegA, N: 0},
		bpf.LoadScratch{Dst: bpf.RegX, N: 0},
		bpf.LoadAbsolute{Off: uint32(nlmsgAlign(0) + unsafe.Sizeof(nlProcResponse{}) + unsafe.Offsetof(exitProcEvent{}.processTgid)), Size: 4},
		bpf.JumpIfX{Cond: bpf.JumpEqual, SkipTrue: 1}, // exit process_pid == process_tgid
		bpf.RetConstant{Val: 0},                       // filter out
		bpf.RetConstant{Val: 0xFFFFFFFF},
		// UID, GID, SID, COMM
		bpf.JumpIf{Cond: bpf.JumpEqual, Val: core.Htonl(uint32(procEventUID)), SkipTrue: 4},  // PROC_EVENT_UID
		bpf.JumpIf{Cond: bpf.JumpEqual, Val: core.Htonl(uint32(procEventGID)), SkipTrue: 3},  // PROC_EVENT_GID
		bpf.JumpIf{Cond: bpf.JumpEqual, Val: core.Htonl(uint32(procEventSID)), SkipTrue: 2},  // PROC_EVENT_SID
		bpf.JumpIf{Cond: bpf.JumpEqual, Val: core.Htonl(uint32(procEventComm)), SkipTrue: 1}, // PROC_EVENT_COMM
		bpf.RetConstant{Val: 0},          // filter out
		bpf.RetConstant{Val: 0xFFFFFFFF}, // UID, GID, SID, COMM
	})
	if err != nil {
		return err
	}

	if _, _, err := syscall.Syscall6(
		syscall.SYS_SETSOCKOPT,
		uintptr(fd),
		uintptr(syscall.SOL_SOCKET),
		uintptr(syscall.SO_ATTACH_FILTER),
		uintptr(unsafe.Pointer(
			&syscall.SockFprog{
				Len:    uint16(len(filter)),
				Filter: (*syscall.SockFilter)(unsafe.Pointer(&filter[0])),
			},
		)),
		unsafe.Sizeof(syscall.SockFprog{}),
		0,
	); err != 0 {
		return err
	}

	return nil
}

// nlProcessListen sets process connector to listen or ignore process messages.
func nlProcessListen(fd int, on bool) error {
	op := procCnMcastIgnore
	if on {
		op = procCnMcastListen
	}
	req := nlProcRequest{
		NlMsghdr: syscall.NlMsghdr{
			Len:  uint32(unsafe.Sizeof(nlProcRequest{})),
			Type: uint16(syscall.NLMSG_DONE),
		},
		cnMsg: cnMsg{
			cbID: cbID{
				idx: cnIdxProc,
				val: cnValProc,
			},
			len: uint16(unsafe.Sizeof(op)),
		},
		procCnMcastOp: op,
	}

	buf := (*[unsafe.Sizeof(req)]byte)(unsafe.Pointer(&req))[:]
	return syscall.Sendto(fd, buf, 0, &syscall.SockaddrNetlink{Family: syscall.AF_NETLINK})
}

// *************************************************
// NETLINK generic TASKSTATS convenience definitions
// *************************************************

// NLA_ALIGNTO from /usr/include/linux/netlink.h
func nlaAlignTo(l int) int {
	return (l + syscall.NLA_ALIGNTO - 1) & ^(syscall.NLA_ALIGNTO - 1)
}

// nlGenlRequest defines netlink generic controller request.
type nlGenlRequest struct {
	syscall.NlMsghdr
	unix.Genlmsghdr
	syscall.NlAttr
}

// nlGeneric opens a socket to the netlink generic controller.
func nlGeneric() (int, error) {
	fd, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_DGRAM, syscall.NETLINK_GENERIC)
	if err != nil {
		return -1, core.NewError("socket", err)
	}
	if err := syscall.Bind(fd,
		&syscall.SockaddrNetlink{
			Family: syscall.AF_NETLINK,
			Groups: 0,
			Pid:    0,
		},
	); err != nil {
		syscall.Close(fd)
		return -1, core.NewError("bind", err)
	}

	return fd, nil
}

// genlFamily gets a family's id by name.
func genlFamily(fd int, name string) (int, error) {
	data := []byte(name + "\x00")
	req := nlGenlRequest{
		NlMsghdr: syscall.NlMsghdr{
			Len:   uint32(syscall.NLMSG_HDRLEN + unix.GENL_HDRLEN + syscall.NLA_HDRLEN + len(data)),
			Type:  unix.GENL_ID_CTRL,
			Flags: syscall.NLM_F_REQUEST,
			Seq:   0,
			Pid:   uint32(os.Getpid()),
		},
		Genlmsghdr: unix.Genlmsghdr{
			Cmd:     unix.CTRL_CMD_GETFAMILY,
			Version: 1,
		},
		NlAttr: syscall.NlAttr{
			Len:  uint16(syscall.NLA_HDRLEN + len(data)),
			Type: unix.CTRL_ATTR_FAMILY_NAME,
		},
	}

	buf := append((*[unsafe.Sizeof(req)]byte)(unsafe.Pointer(&req))[:], data...)
	if err := syscall.Sendto(fd, buf, 0, &syscall.SockaddrNetlink{Family: syscall.AF_NETLINK}); err != nil {
		return -1, err
	}

	nlMsg := make([]byte, 256)
	n, _, err := syscall.Recvfrom(fd, nlMsg, 0)
	if err != nil {
		return -1, err
	}

	attr, err := nlAttr(nlMsg[:n], unix.CTRL_ATTR_FAMILY_ID)
	if err != nil {
		return -1, err
	}

	return int(core.HostEndian.Uint16(attr)), nil
}

// nlAttr finds netlink attribute by key in netlink message.
func nlAttr(nlMsg []byte, key int) ([]byte, error) {
	if len(nlMsg) > 0 {
		msgs, err := syscall.ParseNetlinkMessage(nlMsg)
		if err != nil {
			return nil, err
		}
		for _, m := range msgs {
			if m.Header.Type == syscall.NLMSG_ERROR {
				return nil, syscall.Errno(-int32(core.HostEndian.Uint32(m.Data[:4])))
			}

			data := m.Data[unix.GENL_HDRLEN:]
			var attr *syscall.NlAttr
			for i := 0; i < len(data); i += nlaAlignTo(int(attr.Len)) {
				attr = (*syscall.NlAttr)(unsafe.Pointer(&data[i]))
				if attr.Type == uint16(key) {
					return data[i+4 : i+int(attr.Len)], nil
				}
			}
		}
	}

	return nil, fmt.Errorf("generic netlink family attribute %d unresolved", key)
}

// nlGenericListen initiate listen on netlink generic connector for family's messages.
func nlGenericListen(fd, id int, on bool) error {
	data := []byte("0-" + strconv.Itoa(runtime.NumCPU()-1) + "\x00")
	t := unix.TASKSTATS_CMD_ATTR_DEREGISTER_CPUMASK
	if on {
		t = unix.TASKSTATS_CMD_ATTR_REGISTER_CPUMASK
	}
	req := nlGenlRequest{
		NlMsghdr: syscall.NlMsghdr{
			Len:   uint32(syscall.NLMSG_HDRLEN + unix.GENL_HDRLEN + syscall.NLA_HDRLEN + len(data)),
			Type:  uint16(id), // family id
			Flags: syscall.NLM_F_REQUEST,
			Seq:   0,
			Pid:   uint32(os.Getpid()),
		},
		Genlmsghdr: unix.Genlmsghdr{
			Cmd:     unix.TASKSTATS_CMD_GET,
			Version: 1,
		},
		NlAttr: syscall.NlAttr{
			Len:  uint16(syscall.NLA_HDRLEN + len(data)),
			Type: uint16(t),
		},
	}

	buf := append((*[unsafe.Sizeof(req)]byte)(unsafe.Pointer(&req))[:], data...)
	return syscall.Sendto(fd, buf, 0, &syscall.SockaddrNetlink{Family: syscall.AF_NETLINK})
}

// nlTaskstats queries a specific process for its taskstats.
func nlTaskstats(pid Pid) error {
	data := make([]byte, 4)
	core.HostEndian.PutUint32(data, uint32(pid))
	req := nlGenlRequest{
		NlMsghdr: syscall.NlMsghdr{
			Len:   uint32(syscall.NLMSG_HDRLEN + unix.GENL_HDRLEN + syscall.NLA_HDRLEN + len(data)),
			Type:  uint16(h.id),
			Flags: syscall.NLM_F_REQUEST,
			Seq:   0,
			Pid:   uint32(os.Getpid()),
		},
		Genlmsghdr: unix.Genlmsghdr{
			Cmd:     unix.TASKSTATS_CMD_GET,
			Version: 1,
		},
		NlAttr: syscall.NlAttr{
			Len:  uint16(syscall.NLA_HDRLEN + len(data)),
			Type: unix.TASKSTATS_CMD_ATTR_TGID,
		},
	}

	buf := append((*[unsafe.Sizeof(req)]byte)(unsafe.Pointer(&req))[:], data...)
	return syscall.Sendto(h.gd, buf, 0, &syscall.SockaddrNetlink{Family: syscall.AF_NETLINK})
}
