// Copyright © 2021-2023 The Gomon Project.

package process

import (
	"fmt"
	"sync"
	"time"

	"github.com/zosmac/gocore"
	"github.com/zosmac/gomon/logs"
	"github.com/zosmac/gomon/message"
)

type (
	// Table defines a process table as a map of pids to processes.
	Table = gocore.Table[Pid, *measurement]

	// Tree organizes the processes into a hierarchy
	Tree = gocore.Tree[Pid, int, *measurement]
)

var (
	// clMap caches process command lines, which can be expensive to query.
	clMap  = map[Pid]CommandLine{}
	clLock sync.Mutex

	// oldTimes used to limit reporting only to processes that consumed CPU since the previous measurement.
	oldTimes = map[Pid]time.Duration{}

	// endpoints of processes periodically populated by lsof.
	epMap  = map[Pid][]Connection{}
	epLock sync.RWMutex
)

// Measure captures all processes' metrics.
func Measure() (ProcStats, []message.Content) {
	tb := BuildTable()

	newTimes := map[Pid]time.Duration{}
	for pid, p := range tb {
		newTimes[pid] = p.Total
	}

	var ms []message.Content
	var active, execed int
	var total time.Duration
	for pid, nt := range newTimes {
		if ot, ok := oldTimes[pid]; ok {
			if nt > ot {
				active++
				total += nt - ot
				ms = append(ms, tb[pid])
			}
			delete(oldTimes, pid)
		} else {
			execed++
			if nt > 0 {
				total += nt
				ms = append(ms, tb[pid])
			}
		}
	}

	ps := ProcStats{
		Count:  len(tb),
		Active: active,
		Execed: execed,
		Exited: len(oldTimes),
		CPU:    total,
	}

	var pids []int
	clLock.Lock()
	for pid := range oldTimes { // process exited
		pids = append(pids, int(pid))
		delete(clMap, pid)
	}
	clLock.Unlock()

	logs.Remove(pids)

	oldTimes = newTimes

	return ps, ms
}

// BuildTable builds a process table and captures current process state
func BuildTable() Table {
	pids, err := getPids()
	if err != nil {
		panic(fmt.Errorf("could not build process table %v", err))
	}

	var epm map[Pid][]Connection
	epLock.RLock()
	if len(epMap) > 0 {
		epm = epMap
	}
	epLock.RUnlock()

	tb := make(map[Pid]*measurement, len(pids))
	for _, pid := range pids {
		id, props, metrics := pid.metrics()
		tb[pid] = &measurement{
			Ancestors:   []Pid{},
			Header:      message.Measurement(),
			Id:          id,
			Properties:  props,
			Metrics:     metrics,
			Connections: epm[pid],
		}
	}

	for pid, p := range tb {
		p.Ancestors = func() []Pid {
			var pids []Pid
			for pid = tb[pid].Ppid; pid > 1; pid = tb[pid].Ppid {
				pids = append([]Pid{pid}, pids...)
			}
			return pids
		}()
	}

	return tb
}

// BuildTree builds the process tree.
func BuildTree(tb Table) Tree {
	tr := Tree{}
	for pid := range tb {
		var pids []Pid
		for ; pid > 0; pid = tb[pid].Ppid {
			pids = append([]Pid{pid}, pids...)
		}
		tr.Add(pids...)
	}
	return tr
}
