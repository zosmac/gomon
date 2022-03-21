// Copyright © 2021 The Gomon Project.

package process

import (
	"fmt"
	"time"

	"github.com/zosmac/gomon/core"
	"github.com/zosmac/gomon/message"
)

var (
	// messageChan queues process event observations for periodic reporting.
	messageChan = make(chan *observation, 100)

	// errorChan communicates errors from the observe goroutine.
	errorChan = make(chan error, 10)
)

// Observer starts capture of process event observations.
func Observer() error {
	if err := open(); err != nil {
		return core.Error("process observer", err)
	}

	go observe()

	go func() {
		for {
			select {
			case err := <-errorChan:
				core.LogError(err)
			case obs := <-messageChan:
				message.Encode([]message.Content{obs})
			}
		}
	}()

	return nil
}

// notify assembles a message and queues it.
func notify(id *Id, ev processEvent, msg string) {
	messageChan <- &observation{
		Header:  message.Observation(time.Now(), ev),
		Id:      *id,
		Message: msg,
	}
}

// fork reports a process fork.
func (id *Id) fork() {
	notify(id, processFork, fmt.Sprintf("%s[%d] -> [%d:%s]", id.Name, id.ppid, id.Pid, id.Starttime.Format("20060102-150405")))
}

// exec reports a process exec.
func (id *Id) exec() {
	notify(id, processExec, fmt.Sprintf("[%d] -> %s[%d:%s]", id.ppid, id.Name, id.Pid, id.Starttime.Format("20060102-150405")))
}

// exit reports a process exit.
func (id *Id) exit() {
	notify(id, processExit, fmt.Sprintf("%s[%d:%s]", id.Name, id.Pid, id.Starttime.Format("20060102-150405")))
}

// setuid reports a process change uid. (linux only)
func (id *Id) setuid(uid int) {
	notify(id, processSetuid, fmt.Sprintf("%s[%d] uid: %d", id.Name, id.Pid, uid))
}

// setgid reports a process change gid. (linux only)
func (id *Id) setgid(gid int) {
	notify(id, processSetgid, fmt.Sprintf("%s[%d] gid: %d", id.Name, id.Pid, gid))
}
