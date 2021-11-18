// Copyright © 2021 The Gomon Project.

package process

import (
	"bytes"
	"encoding/binary"
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
	ids map[Pid]id

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
	// start listening for taskstats responses
	if err = nlGenericListen(gd, id, true); err != nil {
		syscall.Close(fd)
		syscall.Close(gd)
		return err
	}

	h = handle{fd: fd, gd: gd, id: id}

	return nil
}

// close stops listening for process events and closes the netlink socket
func (h *handle) close() {
	nlProcessListen(h.fd, false)
	nlGenericListen(h.gd, h.id, false)
	syscall.Close(h.fd)
	syscall.Close(h.gd)
}

// listen for events and notify observer's callbacks.
func listen() {
	go taskstats()
	defer h.close()

	for {
		nlMsg := make([]byte, connectorMaxMessageSize)
		n, _, err := syscall.Recvfrom(h.fd, nlMsg, 0)
		if err != nil {
			errorChan <- core.NewError("recvfrom", err)
			return
		}
		msgs, _ := syscall.ParseNetlinkMessage(nlMsg[:n])

		for _, m := range msgs {
			if m.Header.Type == syscall.NLMSG_ERROR {
				errorChan <- core.NewError("netlink", syscall.Errno(-int32(core.HostEndian.Uint32(m.Data[:4]))))
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
			errorChan <- core.NewError("recvfrom", err)
			return
		}
		msgs, _ := syscall.ParseNetlinkMessage(nlMsg[:n])

		for _, m := range msgs {
			if m.Header.Type == syscall.NLMSG_ERROR {
				errorChan <- core.NewError("netlink", syscall.Errno(-int32(core.HostEndian.Uint32(m.Data[:4]))))
				break
			}

			data := m.Data[unix.GENL_HDRLEN:]
			var attr *syscall.NlAttr

			for i := 0; i < len(data); i += nlaAlignTo(int(attr.Len)) {
				attr = (*syscall.NlAttr)(unsafe.Pointer(&data[i]))
				switch attr.Type {
				case unix.TASKSTATS_TYPE_AGGR_PID, unix.TASKSTATS_TYPE_AGGR_TGID:
					i += syscall.NLA_HDRLEN
					attr = (*syscall.NlAttr)(unsafe.Pointer(&data[i]))
					fallthrough
				case unix.TASKSTATS_TYPE_PID, unix.TASKSTATS_TYPE_TGID:
				case unix.TASKSTATS_TYPE_STATS:
					ts := tsMeasurement{
						Header: message.Measurement(processSource("taskstats")),
					}
					buf := &bytes.Buffer{}
					buf.Write(data[i+syscall.NLA_HDRLEN : i+int(attr.Len)])
					binary.Read(buf, core.HostEndian, (*unix.Taskstats)(unsafe.Pointer(&ts.Taskstats)))
					ts.AcEtime *= time.Microsecond
					ts.AcUtime *= time.Microsecond
					ts.AcStime *= time.Microsecond
					ts.AcUtimescaled *= time.Microsecond
					ts.AcStimescaled *= time.Microsecond
					ts.HiwaterRss *= 1000
					ts.HiwaterVM += 1000

					ts.Btime = time.Unix(int64(ts.AcBtime), 0)
					ts.Command = core.FromCString((*(*[len(ts.AcComm)]int8)(unsafe.Pointer(&ts.AcComm[0])))[:])
					ts.Uname = core.Username(int(ts.AcUID))
					ts.Gname = core.Groupname(int(ts.AcGID))

					message.Encode([]message.Content{&ts})
				}
			}
		}
	}
}
