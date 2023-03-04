// Copyright © 2021-2023 The Gomon Project.

package process

import (
	"syscall"
	"time"
	"unsafe"

	"github.com/zosmac/gocore"
	"github.com/zosmac/gomon/message"
	"golang.org/x/sys/unix"
)

type (
	// handle contains netlink descriptors
	handle struct {
		fd int // netlink process connector descriptor
		gd int // netlink generic connector descriptor
		id int // netlink generic TASKSTATS family id
	}
)

var (
	// h is handle containing netlink descriptors
	h = handle{fd: -1, gd: -1, id: -1}

	// ids maps pids to current process instances.
	ids = map[Pid]Id{}
)

// open obtains netlink socket descriptors.
func open() error {
	// enable the netlink process connector
	fd, err := nlProcess()
	if err != nil {
		return gocore.Error("nlProcess", err)
	}

	// enable the netlink generic connector
	gd, err := nlGeneric()
	if err != nil {
		syscall.Close(fd)
		return gocore.Error("nlGeneric", err)
	}

	// determine the taskstats family id
	id, err := genlFamily(gd, unix.TASKSTATS_GENL_NAME)
	if err != nil {
		syscall.Close(fd)
		syscall.Close(gd)
		return gocore.Error("genlFamily", err)
	}

	// start observing taskstats responses
	if err = nlGenericObserve(gd, id, true); err != nil {
		syscall.Close(fd)
		syscall.Close(gd)
		return gocore.Error("nlGenericObserve", err)
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
func observe() error {
	go taskstats()

	go func() {
		defer h.close()

		for {
			nlMsg := make([]byte, connectorMaxMessageSize)
			n, _, err := syscall.Recvfrom(h.fd, nlMsg, 0)
			if err != nil {
				gocore.LogError("Recvfrom", err)
				return
			}
			msgs, _ := syscall.ParseNetlinkMessage(nlMsg[:n])

			for _, m := range msgs {
				if m.Header.Type == syscall.NLMSG_ERROR {
					gocore.LogError("netlink", syscall.Errno(-int32(gocore.HostEndian.Uint32(m.Data[:4]))))
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
					ids[id.Pid] = id
					id.fork()

				case procEventExec:
					event := (*execProcEvent)(ev)
					pid := Pid(event.processTgid)
					id := pid.id() // get new name
					if i, ok := ids[pid]; ok {
						id.ppid = i.ppid // preserve in case child reassigned to init process
					}
					ids[pid] = id
					id.exec()

				case procEventExit:
					event := (*exitProcEvent)(ev)
					pid := Pid(event.processTgid)
					if id, ok := ids[pid]; ok {
						delete(ids, pid)
						id.exit()
					}

				case procEventUID:
					event := (*uidProcEvent)(ev)
					pid := Pid(event.processTgid)
					id, ok := ids[pid]
					if !ok {
						id = pid.id()
						ids[pid] = id
					}
					id.setuid(int(event.uid))

				case procEventGID:
					event := (*gidProcEvent)(ev)
					pid := Pid(event.processTgid)
					id, ok := ids[pid]
					if !ok {
						id = pid.id()
						ids[pid] = id
					}
					id.setgid(int(event.gid))

				case procEventSID:
					// event := (*sidProcEvent)(ev)
					// if id, ok := ids[Pid(event.processTgid)]; ok {
					// 	fmt.Fprintf(os.Stderr, "SID ========== %#v\n", event)
					// }

				case procEventComm:
					// event := (*commProcEvent)(ev)
					// if id, ok := ids[Pid(event.processTgid)]; ok {
					// 	fmt.Fprintf(os.Stderr, "COMM ========== %#v\n", event)
					// }
				}
			}
		}
	}()

	return nil
}

// taskstats reads netlink process metrics.
func taskstats() {
	for {
		nlMsg := make([]byte, 1024)
		n, _, err := syscall.Recvfrom(h.gd, nlMsg, 0)
		if err != nil {
			gocore.LogError("Recvfrom", err)
			return
		}
		msgs, _ := syscall.ParseNetlinkMessage(nlMsg[:n])

		for _, m := range msgs {
			if m.Header.Type == syscall.NLMSG_ERROR {
				gocore.LogError("netlink", syscall.Errno(-int32(gocore.HostEndian.Uint32(m.Data[:4]))))
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
						Name:      gocore.GoStringN((*byte)(unsafe.Pointer(&tsMsg.ts.Ac_comm[0])), len(tsMsg.ts.Ac_comm)),
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

				ts.Uname = gocore.Username(int(ts.Ac_uid))
				ts.Gname = gocore.Groupname(int(ts.Ac_gid))

				message.Encode([]message.Content{&ts})
			}
		}
	}
}
