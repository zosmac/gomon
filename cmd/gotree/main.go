// Copyright © 2022 The Gomon Project.

package main

/*
#include <libproc.h>
#include <sys/sysctl.h>
*/
import "C"

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"unsafe"

	"github.com/zosmac/gomon/core"
)

var (
	pt processTable
)

type (
	Pid int

	// CommandLine contains a process' command line arguments.
	CommandLine struct {
		Exec string   `json:"exec" gomon:"property"`
		Args []string `json:"args" gomon:"property"`
		Envs []string `json:"envs" gomon:"property"`
	}

	// processTable defines a process table as a map of pids to processes.
	processTable map[Pid]*measurement

	// processTree organizes the process into a hierarchy
	processTree map[Pid]processTree

	// measurement for the message.
	measurement struct {
		ancestors []Pid
		Pid
		Ppid Pid
		CommandLine
	}
)

func main() {
	flatTree(buildTree(buildTable()), 0)
}

// getPids gets the list of active processes by pid.
func getPids() ([]Pid, error) {
	n, err := C.proc_listpids(C.PROC_ALL_PIDS, 0, nil, 0)
	if n <= 0 {
		return nil, core.Error("proc_listpids PROC_ALL_PIDS failed", err)
	}

	var pid C.int
	buf := make([]C.int, n/C.int(unsafe.Sizeof(pid))+10)
	if n, err = C.proc_listpids(C.PROC_ALL_PIDS, 0, unsafe.Pointer(&buf[0]), n); n <= 0 {
		return nil, core.Error("proc_listpids PROC_ALL_PIDS failed", err)
	}
	n /= C.int(unsafe.Sizeof(pid))
	if int(n) < len(buf) {
		buf = buf[:n]
	}

	pids := make([]Pid, len(buf))
	for i, pid := range buf {
		pids[int(n)-i-1] = Pid(pid) // Darwin returns pids in descending order, so reverse the order
	}
	return pids, nil
}

// buildTable builds a process table and captures current process state
func buildTable() processTable {
	pids, err := getPids()
	if err != nil {
		panic(fmt.Errorf("could not build process table %v", err))
	}

	pt = make(map[Pid]*measurement, len(pids))
	for _, pid := range pids {

		var bsi C.struct_proc_bsdshortinfo
		if n := C.proc_pidinfo(
			C.int(pid),
			C.PROC_PIDT_SHORTBSDINFO,
			0,
			unsafe.Pointer(&bsi),
			C.int(C.PROC_PIDT_SHORTBSDINFO_SIZE),
		); n != C.int(C.PROC_PIDT_SHORTBSDINFO_SIZE) {
			continue
		}

		pt[pid] = &measurement{
			ancestors:   []Pid{},
			Pid:         pid,
			Ppid:        Pid(bsi.pbsi_ppid),
			CommandLine: pid.commandLine(),
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
	tab := fmt.Sprintf("\n\t%*s", indent, "")

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
		var cmd, args, envs string
		if len(pt[pid].Args) > 0 {
			cmd = pt[pid].Args[0]
		}
		if len(pt[pid].Args) > 1 {
			args = tab + strings.Join(pt[pid].Args[1:], tab)
		}
		if len(pt[pid].Envs) > 0 {
			envs = tab + strings.Join(pt[pid].Envs, tab)
		}
		p := strconv.Itoa(int(pid))
		pre := "      "[:6-len(p)] + "\033[36;40m" + p
		fmt.Fprintf(os.Stderr, "%*s%s\033[m  %s\033[34m%s\033[35m%s\033[m\n", indent, "", pre, cmd, args, envs)
		flat = append(flat, flatTree(t[pid], indent+3)...)
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

// commandLine retrieves process command, arguments, and environment.
func (pid Pid) commandLine() CommandLine {
	size := C.size_t(C.ARG_MAX)
	buf := make([]byte, size)

	if rv := C.sysctl(
		(*C.int)(unsafe.Pointer(&[3]C.int{C.CTL_KERN, C.KERN_PROCARGS2, C.int(pid)})),
		3,
		unsafe.Pointer(&buf[0]),
		&size,
		unsafe.Pointer(nil),
		0,
	); rv != 0 {
		return CommandLine{}
	}

	l := int(*(*uint32)(unsafe.Pointer(&buf[0])))
	ss := bytes.FieldsFunc(buf[4:size], func(r rune) bool { return r == 0 })
	var exec string
	var args, envs []string
	for i, s := range ss {
		if i == 0 {
			exec = string(s)
		} else if i <= l {
			args = append(args, string(s))
		} else {
			envs = append(envs, string(s))
		}
	}

	return CommandLine{
		Exec: exec,
		Args: args,
		Envs: envs,
	}
}
