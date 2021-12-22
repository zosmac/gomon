// Copyright © 2021 The Gomon Project.

package process

import (
	"sort"
	"sync"
	"time"

	"github.com/zosmac/gomon/log"
	"github.com/zosmac/gomon/message"
)

var (
	// commandLines cache for command lines, which are expensive to query.
	clMap  = map[Pid]CommandLine{}
	clLock sync.RWMutex

	// oldTimes used to limit reporting only to processes that consumed CPU since the previous measurement.
	oldTimes = map[Pid]time.Duration{}

	// endpoints of processes periodically populated by lsof.
	epMap  = map[Pid]Connections{}
	epLock sync.Mutex
)

type (
	// processTable defines a process table as a map of pids to processes.
	processTable map[Pid]*measurement

	// processTree organizes the process into a hierarchy
	processTree map[Pid]processTree
)

// Measure captures all processes' metrics.
func Measure() (ProcStats, []message.Content) {
	pt := buildTable()

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

	log.Remove(pids)

	oldTimes = newTimes

	return ps, ms
}

// buildTable builds a process table and captures current process state
func buildTable() processTable {
	pids, err := getPids()
	if err != nil {
		panic("could not build process table")
	}

	var epm map[Pid]Connections
	epLock.Lock()
	if len(epMap) > 0 {
		epm = epMap
	}
	epLock.Unlock()

	pt := make(map[Pid]*measurement, len(pids))
	for _, pid := range pids {
		id, props, metrics := pid.metrics()
		pt[pid] = &measurement{
			ancestors:   []Pid{},
			Header:      message.Measurement(sourceProcess),
			Id:          id,
			Props:       props,
			Metrics:     metrics,
			Connections: epm[pid],
		}
	}

	for pid, p := range pt {
		p.ancestors = func() []Pid {
			var pids []Pid
			for pid = pt[pid].Ppid; pid > 1; pid = pt[pid].Ppid {
				pids = append([]Pid{pid}, pids...)
			}
			return pids
		}()
	}

	return pt
}

func buildTree(pt processTable) processTree {
	t := processTree{}

	for pid, p := range pt {
		addPid(t, append(p.ancestors, pid))
	}

	return t
}

func addPid(t processTree, ancestors []Pid) {
	if len(ancestors) == 0 {
		return
	}
	if _, ok := t[ancestors[0]]; !ok {
		t[ancestors[0]] = processTree{}
	}
	addPid(t[ancestors[0]], ancestors[1:])
}

func flatTree(t processTree, indent int) []Pid {
	var flat []Pid

	pids := make([]Pid, len(t))
	var i int
	for pid := range t {
		pids[i] = pid
		i++
	}

	sort.Slice(pids, func(i, j int) bool {
		dti := depthTree(t[pids[i]])
		dtj := depthTree(t[pids[j]])
		return dti > dtj ||
			dti == dtj && pids[i] < pids[j]
	})

	for _, pid := range pids {
		flat = append(flat, pid)
		// fmt.Fprintf(os.Stderr, "%*s%6d\n", indent, "", pid)
		flat = append(flat, flatTree(t[pid], indent+2)...)
	}

	return flat
}

func depthTree(t processTree) int {
	depth := 0
	for _, tree := range t {
		dt := depthTree(tree) + 1
		if depth < dt {
			depth = dt
		}
	}
	return depth
}

func findTree(t processTree, pid Pid) processTree {
	if t, ok := t[pid]; ok {
		return t
	}
	for _, t := range t {
		if findTree(t, pid) != nil {
			return t
		}
	}

	return nil
}
