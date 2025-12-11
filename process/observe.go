// Copyright © 2021-2023 The Gomon Project.

package process

import (
	"context"
	"fmt"
	"time"

	"github.com/zosmac/gomon/message"
)

var (
	// messageChan queues process event observations for periodic reporting.
	messageChan = make(chan *observation, 100)
)

// Observer starts capture of process event observations.
func Observer(ctx context.Context) error {
	if err := open(); err != nil {
		return err
	}

	if err := observe(); err != nil {
		return err
	}

	go func() {
		for obs := range messageChan {
			message.Observations([]message.Content{obs})
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
