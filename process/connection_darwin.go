// Copyright © 2021 The Gomon Project.

package process

/*
#include <libproc.h>
#include <sys/event.h>
#include <sys/fcntl.h>
#include <sys/kern_control.h>
#include <sys/sysctl.h>
#include <sys/un.h>
// from https://github.com/apple/darwin-xnu/blob/master/bsd/sys/file_internal.h
typedef enum {
	DTYPE_VNODE = 1,
	DTYPE_SOCKET,
	DTYPE_PSXSHM,
	DTYPE_PSXSEM,
	DTYPE_KQUEUE,
	DTYPE_PIPE,
	DTYPE_FSEVENTS,
	DTYPE_ATALK,
	DTYPE_NETPOLICY,
	DTYPE_CHAN,
	DTYPE_NEXUS
} file_type_t;

// undocumented in proc_info.h, but shown by lsof command
#define PROX_FDTYPE_CHAN 10
#define PROX_FDTYPE_NEXUS 11
*/
import "C"

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"strconv"
	"syscall"
	"unsafe"

	"github.com/zosmac/gomon/core"
)

var (
	fdinfos = func() []C.struct_proc_fdinfo {
		var rlimit syscall.Rlimit
		syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rlimit)
		return make([]C.struct_proc_fdinfo, rlimit.Cur)
	}()
)

// getInodes is called from Measure(), but is only relevant for Linux, so for Darwin is a noop.
func getInodes() {}

// endpoints returns a list of Connection structures for a process.
func (pid Pid) endpoints() Connections {
	n := C.proc_pidinfo(
		C.int(pid),
		C.PROC_PIDLISTFDS,
		0,
		unsafe.Pointer(&fdinfos[0]),
		C.int(len(fdinfos))*C.PROC_PIDLISTFD_SIZE,
	)
	if n <= 0 {
		return nil
	}
	n /= C.PROC_PIDLISTFD_SIZE

	conns := make(Connections, n)
	for i, fdinfo := range fdinfos[:n] {
		conns[i] = fdConn(pid, fdinfo)
	}

	return conns
}

// fdConn determines the connection type for a given file descriptor, given the C proc_fdinfo
// structure for the fdtype defined in /usr/include/sys/proc_info.h.
func fdConn(pid Pid, fdinfo C.struct_proc_fdinfo) Connection {
	fd := int(fdinfo.proc_fd)
	var flavor, size C.int

	switch fdinfo.proc_fdtype {
	case C.PROX_FDTYPE_VNODE:
		flavor = C.PROC_PIDFDVNODEPATHINFO
		size = C.PROC_PIDFDVNODEPATHINFO_SIZE
	case C.PROX_FDTYPE_SOCKET:
		flavor = C.PROC_PIDFDSOCKETINFO
		size = C.PROC_PIDFDSOCKETINFO_SIZE
	case C.PROX_FDTYPE_PSEM:
		flavor = C.PROC_PIDFDPSEMINFO
		size = C.PROC_PIDFDPSEMINFO_SIZE
	case C.PROX_FDTYPE_PSHM:
		flavor = C.PROC_PIDFDPSHMINFO
		size = C.PROC_PIDFDPSHMINFO_SIZE
	case C.PROX_FDTYPE_PIPE:
		flavor = C.PROC_PIDFDPIPEINFO
		size = C.PROC_PIDFDPIPEINFO_SIZE
	case C.PROX_FDTYPE_KQUEUE:
		flavor = C.PROC_PIDFDKQUEUEINFO
		size = C.PROC_PIDFDKQUEUEINFO_SIZE
		// flavor = C.PROC_PIDFDKQUEUE_EXTINFO // TODO: test if this flavor extracts more info
		// size = C.PROC_PIDFDKQUEUE_EXTINFO_SIZE
	default:
		return otherConn(pid, fdinfo)
	}

	buf := make([]byte, size)

	if n := C.proc_pidfdinfo(
		C.int(pid),
		C.int(fd),
		flavor,
		unsafe.Pointer(&buf[0]),
		size,
	); n <= 0 {
		return Connection{
			Descriptor: fd,
			Type:       fmt.Sprintf("%d", fdinfo.proc_fdtype),
			Name:       "UNRESOLVABLE",
		}
	}

	pfi := (*C.struct_proc_fileinfo)(unsafe.Pointer(&buf[0]))

	switch pfi.fi_type {
	case C.DTYPE_VNODE:
		return fileConn(fd, pfi)
	case C.DTYPE_SOCKET:
		return sockConn(fd, pfi)
	case C.DTYPE_PSXSHM:
		return pshmConn(fd, pfi)
	case C.DTYPE_PSXSEM:
		return psemConn(fd, pfi)
	case C.DTYPE_KQUEUE:
		return kqueueConn(fd, pfi)
	case C.DTYPE_PIPE:
		return pipeConn(fd, pfi)
	default:
		return otherConn(pid, fdinfo)
	}
}

// accmode determines the I/O direction.
func accmode(pfi *C.struct_proc_fileinfo) string {
	flags := pfi.fi_openflags
	switch flags & (C.FREAD | C.FWRITE) {
	case C.FREAD | C.FWRITE:
		return "<-->"
	case C.FREAD:
		return "<<--"
	case C.FWRITE:
		return "-->>"
	}
	return ""
}

// fileConn
func fileConn(fd int, pfi *C.struct_proc_fileinfo) Connection {
	pvip := &(*C.struct_vnode_fdinfowithpath)(unsafe.Pointer(pfi)).pvip
	conn := Connection{
		Descriptor: fd,
		Name:       C.GoString(&pvip.vip_path[0]),
		Direction:  accmode(pfi),
	}
	switch pvip.vip_vi.vi_stat.vst_mode & C.S_IFMT {
	case C.S_IFBLK:
		conn.Type = "BLK"
	case C.S_IFCHR:
		conn.Type = "CHR"
	case C.S_IFDIR:
		conn.Type = "DIR"
	case C.S_IFREG:
		conn.Type = "REG"
	case C.S_IFLNK:
		conn.Type = "LINK"
	case C.S_IFIFO:
		conn.Type = "FIFO"
	case C.S_IFSOCK:
		conn.Type = "SOCK"
	}

	if conn.Name == os.DevNull {
		conn.Type = "NUL"
		return conn
	}

	if conn.Type == "FIFO" && conn.Direction == "-->>" {
		conn.Peer = fmt.Sprintf("%#.x", pvip.vip_vi.vi_stat.vst_ino)
	} else {
		conn.Self = fmt.Sprintf("%#.x", pvip.vip_vi.vi_stat.vst_ino)
	}

	return conn
}

// sockConn
func sockConn(fd int, pfi *C.struct_proc_fileinfo) Connection {
	psi := &(*C.struct_socket_fdinfo)(unsafe.Pointer(pfi)).psi
	conn := Connection{
		Descriptor: fd,
		Direction:  accmode(pfi),
	}
	switch psi.soi_kind {
	case C.SOCKINFO_IN, C.SOCKINFO_TCP:
		switch p := psi.soi_protocol; p {
		case C.IPPROTO_IP:
			conn.Type = "IP"
		case C.IPPROTO_ICMP:
			conn.Type = "ICMP"
		case C.IPPROTO_ICMPV6:
			conn.Type = "ICMPV6"
		case C.IPPROTO_IGMP:
			conn.Type = "IGMP"
		case C.IPPROTO_TCP:
			conn.Type = "TCP"
		case C.IPPROTO_UDP:
			conn.Type = "UDP"
		default:
			conn.Type = fmt.Sprintf("INET(%d)", p)
		}

		in := (*C.struct_in_sockinfo)(unsafe.Pointer(&psi.soi_proto[0]))
		laddr := make([]byte, 16)
		copy(laddr, (*[16]byte)(unsafe.Pointer(&in.insi_laddr[0]))[:])
		if bytes.Equal(laddr[:12], []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}) {
			laddr = laddr[12:]
		}
		conn.Self = net.JoinHostPort(net.IP(laddr).String(), strconv.Itoa(int(core.Ntohs(uint16(in.insi_lport)))))

		if in.insi_fport != 0 {
			faddr := make([]byte, 16)
			copy(faddr, (*[16]byte)(unsafe.Pointer(&in.insi_faddr[0]))[:])
			if bytes.Equal(faddr[:12], []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}) {
				faddr = faddr[12:]
			}
			conn.Peer = net.JoinHostPort(net.IP(faddr).String(), strconv.Itoa(int(core.Ntohs(uint16(in.insi_fport)))))
		}

		if conn.Peer == "" {
			conn.Name = conn.Self + " (listen)"
			conn.Direction = "<<--"
		} else {
			conn.Name = conn.Peer + " -> " + conn.Self
		}

	case C.SOCKINFO_UN:
		conn.Type = "UNIX"

		un := (*C.struct_un_sockinfo)(unsafe.Pointer(&psi.soi_proto[0]))
		conn.Name = C.GoString(&(*C.struct_sockaddr_un)(unsafe.Pointer(&un.unsi_addr)).sun_path[0])
		if conn.Name == "" {
			conn.Name = C.GoString(&(*C.struct_sockaddr_un)(unsafe.Pointer(&un.unsi_caddr)).sun_path[0])
		}
		if conn.Name == "" {
			conn.Name = "socketpair"
		}
		conn.Self = fmt.Sprintf("%#.x", psi.soi_pcb)
		// Peer for a unixConn is zero for a "listener", but unlike for TCP, a new socket is not
		conn.Peer = fmt.Sprintf("%#.x", un.unsi_conn_pcb)

	case C.SOCKINFO_KERN_EVENT:
		conn.Type = "KEVT"
		evt := (*C.struct_kern_event_info)(unsafe.Pointer(&psi.soi_proto[0]))
		conn.Name = fmt.Sprintf("%#.x:%#.x:%#.x", evt.kesi_vendor_code_filter, evt.kesi_class_filter, evt.kesi_subclass_filter)
		conn.Self = fmt.Sprintf("%#.x", psi.soi_pcb)
		conn.Peer = "kernel"

	case C.SOCKINFO_KERN_CTL:
		conn.Type = "KCTL"
		ctl := (*C.struct_kern_ctl_info)(unsafe.Pointer(&psi.soi_proto[0]))
		conn.Name = fmt.Sprintf("%s %d %d", C.GoString(&ctl.kcsi_name[0]), ctl.kcsi_id, ctl.kcsi_unit)
		conn.Self = fmt.Sprintf("%#.x", psi.soi_pcb)
		conn.Peer = "kernel"

	case C.SOCKINFO_NDRV:
		conn.Type = "SOCK:NDRV"
	}

	return conn
}

// pipeConn
func pipeConn(fd int, pfi *C.struct_proc_fileinfo) Connection {
	pipeinfo := &(*C.struct_pipe_fdinfo)(unsafe.Pointer(pfi)).pipeinfo
	return Connection{
		Descriptor: fd,
		Type:       "PIPE",
		Name:       fmt.Sprintf("%#.x", pipeinfo.pipe_stat.vst_ino),
		Direction:  accmode(pfi),
		Self:       fmt.Sprintf("%#.x", pipeinfo.pipe_handle),
		Peer:       fmt.Sprintf("%#.x", pipeinfo.pipe_peerhandle),
	}
}

// pshmConn
func pshmConn(fd int, pfi *C.struct_proc_fileinfo) Connection {
	pshminfo := &(*C.struct_pshm_fdinfo)(unsafe.Pointer(pfi)).pshminfo
	conn := Connection{
		Descriptor: fd,
		Type:       "SHMEM",
		Name:       C.GoString(&pshminfo.pshm_name[0]),
		Direction:  accmode(pfi),
	}

	if conn.Direction != "<<--" {
		conn.Self = conn.Name
	}
	if conn.Direction == "<<--" {
		conn.Peer = conn.Name
	}
	return conn
}

// psemConn
func psemConn(fd int, pfi *C.struct_proc_fileinfo) Connection {
	pseminfo := &(*C.struct_psem_fdinfo)(unsafe.Pointer(pfi)).pseminfo
	return Connection{
		Descriptor: fd,
		Type:       "SEMA",
		Name:       C.GoString(&pseminfo.psem_name[0]),
		Direction:  accmode(pfi),
		Self:       fmt.Sprintf("%#.x", pseminfo.psem_stat.vst_ino),
		Peer:       fmt.Sprintf("%#.x", pseminfo.psem_stat.vst_ino),
	}
}

// kqueueConn (any way to get "dyninfo", which has more fields?)
func kqueueConn(fd int, pfi *C.struct_proc_fileinfo) Connection {
	// kqueueinfo := &(*C.struct_kqueue_fdinfo)(unsafe.Pointer(pfi)).kqueueinfo /* *C.struct_kqueue_dyninfo ?? */
	return Connection{
		Descriptor: fd,
		Type:       "KQUEUE",
		Direction:  accmode(pfi),
	}
}

// otherConn
func otherConn(pid Pid, fdinfo C.struct_proc_fdinfo) Connection {
	var t string
	switch fdinfo.proc_fdtype {
	case C.PROX_FDTYPE_ATALK:
		t = "ATALK"
	case C.PROX_FDTYPE_FSEVENTS:
		t = "FSEVENTS"
	case C.PROX_FDTYPE_NETPOLICY:
		t = "NETPOLICY"
	case C.PROX_FDTYPE_CHAN:
		t = "CHAN"
	case C.PROX_FDTYPE_NEXUS:
		t = "NEXUS"
	default:
		t = "UNRECOGNIZED"
		core.LogError(fmt.Errorf("proc_fdinfo pid %d, fd %d: unrecognized fdtype %d", pid, fdinfo.proc_fd, fdinfo.proc_fdtype))
	}
	return Connection{
		Descriptor: int(fdinfo.proc_fd),
		Type:       t,
	}
}

// ==============================================================================

// status returns the status flag
// func status(conn Connection) string {
// 	flags := (*C.struct_proc_fileinfo)(unsafe.Pointer(reflect.ValueOf(conn).Pointer())).fi_status
// 	var status []string
// 	if flags&C.PROC_FP_SHARED != 0 {
// 		status = append(status, "SHARED")
// 	}
// 	if flags&C.PROC_FP_CLEXEC != 0 {
// 		status = append(status, "CLEXEC")
// 	}
// 	if flags&C.PROC_FP_GUARDED != 0 {
// 		status = append(status, "GUARDED")
// 	}
// 	if flags&C.PROC_FP_CLFORK != 0 {
// 		status = append(status, "CLFORK")
// 	}
// 	return strings.Join(status, ",")
// }

// guard returns the guard flag
// func guard(conn Connection) string {
// 	flags := (*C.struct_proc_fileinfo)(unsafe.Pointer(reflect.ValueOf(conn).Pointer())).fi_guardflags
// 	var guard []string
// 	if flags&C.PROC_FI_GUARD_CLOSE != 0 {
// 		guard = append(guard, "CLOSE")
// 	}
// 	if flags&C.PROC_FI_GUARD_DUP != 0 {
// 		guard = append(guard, "DUP")
// 	}
// 	if flags&C.PROC_FI_GUARD_SOCKET_IPC != 0 {
// 		guard = append(guard, "SOCKET_IPC")
// 	}
// 	if flags&C.PROC_FI_GUARD_FILEPORT != 0 {
// 		guard = append(guard, "FILEPORT")
// 	}
// 	return strings.Join(guard, ",")
// }

// kqs returns a list of kqueue connections for a process.
// func kqs(pid int) map[int]Connection {
// 	n := C.proc_list_dynkqueueids(C.int(pid), nil, 0)
// 	if n <= 0 {
// 		return nil
// 	}

// 	buf := make([]byte, n)
// 	n = proc_list_dynkqueueids(
// 		C.int(pid),
// 		unsafe.Pointer(&buf[0]),
// 		n,
// 	)
// 	if n <= 0 {
// 		return nil
// 	}
// 	if int(n) < len(buf) {
// 		buf = buf[:n:n]
// 	}

// 	conns := map[int]Connection{}
// 	// for i := C.int(0); i < n; i += unsafe.Sizeof(C.kqueue_id_t) {
// 	for i := C.int(0); i < n; i += C.sizeof_kqueue_id_t {
// 		if kq, conn := kqConn(pid, (*C.kqueue_id_t)(unsafe.Pointer(&buf[i]))); conn != nil {
// 			// conns[kq] = conn
// 		}
// 	}

// 	return conns
// }

// // kqConn returns the kqueue_id_t and kqueue_dyninfo structure for a kqueue defined in /usr/include/sys/proc_info.h
// func kqConn(pid int, kqueue_id *C.kqueue_id_t) (kq int, conn Connection) {
// 	kq = int(*kqueue_id)
// 	flavor := C.PROC_PIDFDKQUEUE_EXTINFO
// 	size := C.PROC_PIDFDKQUEUE_EXTINFO_SIZE
// 	buf := make([]byte, size)

// 	n, err := C.proc_piddynkqueueinfo(C.int(pid), flavor, *C.kqueue_id, unsafe.Pointer(&buf[0]), size)
// 	if n == 0 {
// 		core.LogError(fmt.Errorf("proc_piddynkqueueinfo(%d,%d,%d): %v\n", pid, flavor, kq, err))
// 		return
// 	}

// 	return
// }
