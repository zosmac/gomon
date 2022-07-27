// Copyright © 2021 The Gomon Project.

package process

import (
	"syscall"
	"time"
	"unsafe"

	"github.com/zosmac/gomon/core"
	"github.com/zosmac/gomon/message"
	"golang.org/x/sys/unix"
)

var (
	// h is handle containing netlink descriptors
	h = handle{fd: -1, gd: -1, id: -1}

	// youth tracks processes spawned after gomon starts
	youth = ids{}
)

type (
	ids map[Pid]Id

	// handle contains netlink descriptors
	handle struct {
		fd int // netlink process connector descriptor
		gd int // netlink generic connector descriptor
		id int // netlink generic TASKSTATS family id
	}
)

// open obtains netlink socket descriptors.
func open() error {
	// enable the netlink process connector
	fd, err := nlProcess()
	if err != nil {
		return err
	}

	// enable the netlink generic connector
	gd, err := nlGeneric()
	if err != nil {
		syscall.Close(fd)
		return err
	}
	// determine the taskstats family id
	id, err := genlFamily(gd, unix.TASKSTATS_GENL_NAME)
	if err != nil {
		syscall.Close(fd)
		syscall.Close(gd)
		return err
	}
	// start observing taskstats responses
	if err = nlGenericObserve(gd, id, true); err != nil {
		syscall.Close(fd)
		syscall.Close(gd)
		return err
	}

	h = handle{fd: fd, gd: gd, id: id}

	return nil
}

// close stops observing process events and closes the netlink socket
func (h *handle) close() {
	nlProcessObserve(h.fd, false)
	nlGenericObserve(h.gd, h.id, false)
	syscall.Close(h.fd)
	syscall.Close(h.gd)
}

// observe events and notify observer's callbacks.
func observe() {
	// start the lsof command as a sub-process to list periodically all open files and network connections.
	if err := lsofCommand(); err != nil {
		core.LogError(err)
	}

	go taskstats()

	defer h.close()

	for {
		nlMsg := make([]byte, connectorMaxMessageSize)
		n, _, err := syscall.Recvfrom(h.fd, nlMsg, 0)
		if err != nil {
			errorChan <- core.Error("recvfrom", err)
			return
		}
		msgs, _ := syscall.ParseNetlinkMessage(nlMsg[:n])

		for _, m := range msgs {
			if m.Header.Type == syscall.NLMSG_ERROR {
				errorChan <- core.Error("netlink", syscall.Errno(-int32(core.HostEndian.Uint32(m.Data[:4]))))
				break
			}

			if m.Header.Type != syscall.NLMSG_DONE {
				continue
			}

			hdr := (*procEvent)(unsafe.Pointer(&m.Data[unsafe.Sizeof(cnMsg{})]))
			ev := unsafe.Pointer(&m.Data[unsafe.Sizeof(cnMsg{})+unsafe.Sizeof(procEvent{})])

			switch hdr.what {
			case procEventFork:
				event := (*forkProcEvent)(ev)
				ppid := Pid(event.parentTgid)
				pid := Pid(event.childTgid)
				id := pid.id()
				id.ppid = ppid // preserve in case child reassigned to init process
				youth[id.Pid] = id
				id.fork()

			case procEventExec:
				event := (*execProcEvent)(ev)
				pid := Pid(event.processTgid)
				id := pid.id() // get new name
				if i, ok := youth[pid]; ok {
					id.ppid = i.ppid // preserve in case child reassigned to init process
				}
				youth[pid] = id
				id.exec()

			case procEventExit:
				event := (*exitProcEvent)(ev)
				pid := Pid(event.processTgid)
				if id, ok := youth[pid]; ok {
					delete(youth, pid)
					id.exit()
				}

			case procEventUID:
				event := (*uidProcEvent)(ev)
				pid := Pid(event.processTgid)
				id, ok := youth[pid]
				if !ok {
					id = pid.id()
					youth[pid] = id
				}
				id.setuid(int(event.uid))

			case procEventGID:
				event := (*gidProcEvent)(ev)
				pid := Pid(event.processTgid)
				id, ok := youth[pid]
				if !ok {
					id = pid.id()
					youth[pid] = id
				}
				id.setgid(int(event.gid))

			case procEventSID:
				// event := (*sidProcEvent)(ev)
				// if id, ok := youth[Pid(event.processTgid)]; ok {
				// 	fmt.Fprintf(os.Stderr, "SID ========== %+v\n", event)
				// }

			case procEventComm:
				// event := (*commProcEvent)(ev)
				// if id, ok := youth[Pid(event.processTgid)]; ok {
				// 	fmt.Fprintf(os.Stderr, "COMM ========== %+v\n", event)
				// }
			}
		}
	}
}

// taskstats reads netlink process metrics.
func taskstats() {
	for {
		nlMsg := make([]byte, 1024)
		n, _, err := syscall.Recvfrom(h.gd, nlMsg, 0)
		if err != nil {
			errorChan <- core.Error("recvfrom", err)
			return
		}
		msgs, _ := syscall.ParseNetlinkMessage(nlMsg[:n])

		for _, m := range msgs {
			if m.Header.Type == syscall.NLMSG_ERROR {
				errorChan <- core.Error("netlink", syscall.Errno(-int32(core.HostEndian.Uint32(m.Data[:4]))))
				break
			}
			data := m.Data[unix.GENL_HDRLEN:]

			var tsMsg *struct {
				attr1 syscall.NlAttr
				attr2 syscall.NlAttr
				pid   uint32
				attr3 syscall.NlAttr
				ts    unix.Taskstats
			}

			for i := 0; i < len(data); i += nlaAlignTo(int(tsMsg.attr1.Len)) {
				*(*uintptr)(unsafe.Pointer(&tsMsg)) = uintptr(unsafe.Pointer(&data[i]))
				if tsMsg.attr1.Type != unix.TASKSTATS_TYPE_AGGR_PID && tsMsg.attr1.Type != unix.TASKSTATS_TYPE_AGGR_TGID &&
					tsMsg.attr2.Type != unix.TASKSTATS_TYPE_PID && tsMsg.attr2.Type != unix.TASKSTATS_TYPE_TGID &&
					tsMsg.attr3.Type != unix.TASKSTATS_TYPE_STATS {
					continue
				}

				ts := tsMeasurement{
					Header: message.Observation(time.Now(), netlinkTaskstats),
					Id: Id{
						ppid:      Pid(tsMsg.ts.Ac_ppid),
						Name:      core.FromCString((*(*[len(tsMsg.ts.Ac_comm)]int8)(unsafe.Pointer(&tsMsg.ts.Ac_comm[0])))[:]),
						Pid:       Pid(tsMsg.pid),
						Starttime: time.Unix(int64(tsMsg.ts.Ac_btime), 0),
					},
				}

				ts.Taskstats = tsMsg.ts // *(*Taskstats)(unsafe.Pointer(&tsMsg.ts))
				// ts.AcEtime *= time.Microsecond
				// ts.AcUtime *= time.Microsecond
				// ts.AcStime *= time.Microsecond
				// ts.AcUtimescaled *= time.Microsecond
				// ts.AcStimescaled *= time.Microsecond
				// ts.HiwaterRss *= 1024
				// ts.HiwaterVM *= 1024

				ts.Uname = core.Username(int(ts.Ac_uid))
				ts.Gname = core.Groupname(int(ts.Ac_gid))

				message.Encode([]message.Content{&ts})
			}
		}
	}
}
