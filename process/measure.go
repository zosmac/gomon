// Copyright © 2021-2023 The Gomon Project.

package process

import (
	"fmt"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/zosmac/gocore"
	"github.com/zosmac/gomon/logs"
	"github.com/zosmac/gomon/message"
)

type (
	// Table defines a process table as a map of pids to processes.
	Table = gocore.Table[Pid, *Process]

	// Tree organizes the process pids into a hierarchy.
	Tree = gocore.Tree[Pid]
)

var (
	// clMap caches process command lines, which can be expensive to query.
	clMap  = map[Pid]CommandLine{}
	clLock sync.Mutex

	// procs contains the process table recreated with each measurement.
	procs    = Table{}
	procLock sync.RWMutex

	// prevProcs and pms used to report only processes that are currently and haveconsumed CPU since the previous measurement.
	prevProcs = Table{}
	pms       = []message.Content{}
)

// Measure captures all processes' metrics.
func Measure() (ProcStats, []message.Content) {
	procLock.Lock()
	prevProcs = procs
	procs = buildTable()
	tb := procs
	ptb := prevProcs
	procLock.Unlock()

	if len(prevProcs) == 0 { // await until diffs can be computed
		return ProcStats{}, nil
	}

	exited := map[Pid]struct{}{}
	diffCPU := map[Pid]time.Duration{}
	var cpus []time.Duration
	for pid := range ptb {
		exited[pid] = struct{}{}
	}

	var active, execed int
	var total time.Duration
	for pid, p := range tb {
		if pp, ok := ptb[pid]; ok {
			if diff := p.Total - pp.Total; diff > 0 {
				diffCPU[pid] = diff
				cpus = append(cpus, diff)
				active++
				total += diff
			}
			delete(exited, pid)
		} else {
			diffCPU[pid] = p.Total
			cpus = append(cpus, p.Total)
			active++
			execed++
			total += p.Total
		}
	}

	var ms []message.Content
	slices.Sort(cpus)
	var minCPU time.Duration
	if len(cpus) > int(flags.top) {
		minCPU = cpus[len(cpus)-int(flags.top)-1]
	} else {
		minCPU = cpus[0]
	}
	for pid, cpu := range diffCPU {
		if cpu > minCPU {
			ms = append(ms, tb[pid])
		}
	}

	tops := map[Pid]struct{}{}
	for _, m := range ms {
		tops[m.(*measurement).EventID.Pid] = struct{}{}
	}
	for _, m := range pms {
		pid := m.(*measurement).EventID.Pid
		if _, ok := tops[pid]; ok {
			continue
		}
		if p, ok := tb[pid]; ok {
			if pp, ok := ptb[pid]; ok {
				if diff := p.Total - pp.Total; diff > 0 {
					ms = append(ms, m)
				}
			}
		}
	}
	pms = ms

	ps := ProcStats{
		Count:  len(tb),
		Active: active,
		Execed: execed,
		Exited: len(exited),
		CPU:    total,
	}

	gocore.Error("Process Measure", nil, map[string]string{
		"total":  strconv.Itoa(ps.Count),
		"active": strconv.Itoa(ps.Active),
		"execed": strconv.Itoa(ps.Execed),
		"exited": strconv.Itoa(ps.Exited),
		"CPU":    total.String(),
	}).Info()

	var pids []int
	clLock.Lock()
	for pid := range exited { // process exited
		pids = append(pids, int(pid))
		delete(clMap, pid)
	}
	clLock.Unlock()

	logs.Remove(pids)

	return ps, ms
}

// buildTable builds a process table and captures current process state.
func buildTable() Table {
	pids, err := getPids()
	if err != nil {
		panic(fmt.Errorf("could not build process table %v", err))
	}

	epLock.RLock()
	epm := epMap
	epLock.RUnlock()

	tb := make(map[Pid]*measurement, len(pids))
	for _, pid := range pids {
		id, props, metrics := pid.metrics()
		tb[pid] = &measurement{
			Header:      message.Measurement(),
			EventID:     id,
			Properties:  props,
			Metrics:     metrics,
			Connections: epm[pid],
		}
	}

	// getTaskInfo(tb)
	return tb
}

func (p *Process) HasParent() bool {
	return p.Ppid > 0
}

func (p *Process) Parent() (pid Pid) {
	if p.Ppid > 0 {
		pid = p.Ppid
	}
	return
}
