// Copyright © 2021-2023 The Gomon Project.

package process

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/zosmac/gomon/logs"
	"github.com/zosmac/gomon/message"
)

type (
	// Table defines a process table as a map of pids to processes.
	Table map[Pid]*measurement

	// Tree organizes the processes into a hierarchy
	Tree map[Pid]Tree
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
	pt := BuildTable()

	newTimes := map[Pid]time.Duration{}
	for pid, p := range pt {
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
				ms = append(ms, pt[pid])
			}
			delete(oldTimes, pid)
		} else {
			execed++
			if nt > 0 {
				total += nt
				ms = append(ms, pt[pid])
			}
		}
	}

	ps := ProcStats{
		Count:  len(pt),
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

	pt := make(map[Pid]*measurement, len(pids))
	for _, pid := range pids {
		id, props, metrics := pid.metrics()
		pt[pid] = &measurement{
			Ancestors:   []Pid{},
			Header:      message.Measurement(),
			Id:          id,
			Properties:  props,
			Metrics:     metrics,
			Connections: epm[pid],
		}
	}

	for pid, p := range pt {
		p.Ancestors = func() []Pid {
			var pids []Pid
			for pid = pt[pid].Ppid; pid > 1; pid = pt[pid].Ppid {
				pids = append([]Pid{pid}, pids...)
			}
			return pids
		}()
	}

	return pt
}

func BuildTree(pt Table) Tree {
	tr := Tree{}

	for pid, p := range pt {
		var ancestors []Pid
		for pid := p.Ppid; pid > 1; pid = pt[pid].Ppid {
			ancestors = append([]Pid{pid}, ancestors...)
		}
		addPid(tr, append(ancestors, pid))
	}

	return tr
}

func addPid(tr Tree, ancestors []Pid) {
	if len(ancestors) == 0 {
		return
	}
	if _, ok := tr[ancestors[0]]; !ok {
		tr[ancestors[0]] = Tree{}
	}
	addPid(tr[ancestors[0]], ancestors[1:])
}

func FlatTree(tr Tree) []Pid {
	return flatTree(tr, 0)
}

func flatTree(tr Tree, indent int) []Pid {
	if len(tr) == 0 {
		return nil
	}
	var flat []Pid

	var pids []Pid
	for pid := range tr {
		pids = append(pids, pid)
	}

	sort.Slice(pids, func(i, j int) bool {
		dti := depthTree(tr[pids[i]])
		dtj := depthTree(tr[pids[j]])
		return dti > dtj ||
			dti == dtj && pids[i] < pids[j]
	})

	for _, pid := range pids {
		flat = append(flat, pid)
		flat = append(flat, flatTree(tr[pid], indent+3)...)
	}

	return flat
}

// depthTree enables sort of deepest process trees first.
func depthTree(tr Tree) int {
	depth := 0
	for _, tr := range tr {
		dt := depthTree(tr) + 1
		if depth < dt {
			depth = dt
		}
	}
	return depth
}

// FindTree finds the process tree parented by a specific process.
func FindTree(tr Tree, parent Pid) Tree {
	for pid, tr := range tr {
		if pid == parent {
			return Tree{parent: tr}
		}
		if tr = FindTree(tr, parent); tr != nil {
			return tr
		}
	}

	return nil
}
