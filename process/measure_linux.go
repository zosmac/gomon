// Copyright © 2021 The Gomon Project.

package process

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/zosmac/gomon/core"
)

var (
	// status maps status codes to state names.
	status = map[byte]string{
		'R': "Running",
		'S': "Sleeping",
		'D': "Waiting",
		'Z': "Zombie",
		'T': "Stopped",
		'X': "Dead",
	}

	// factor is the system units for CPU time (i.e. "ticks" or "jiffies").
	factor = 10000 * time.Microsecond
)

// id captures the process identifier.
func (pid Pid) id() Id {
	buf, err := os.ReadFile(filepath.Join("/proc", pid.String(), "stat"))
	if err != nil {
		core.LogError(core.Error("ReadFile", err))
		return Id{}
	}

	fields := strings.Fields(string(buf))
	ppid, _ := strconv.Atoi(fields[3])
	start, _ := strconv.Atoi(fields[21])

	return Id{
		ppid:      Pid(ppid),
		Name:      fields[1][1 : len(fields[1])-1],
		Pid:       pid,
		Starttime: core.Boottime.Add(time.Duration(start) * factor),
	}
}

// metrics captures the metrics for a process.
func (pid Pid) metrics() (Id, Properties, Metrics) {
	buf, err := os.ReadFile(filepath.Join("/proc", pid.String(), "stat"))
	if err != nil {
		core.LogError(core.Error("ReadFile", err))
		return Id{Pid: pid}, Properties{}, Metrics{}
	}
	fields := strings.Fields(string(buf))

	m, _ := core.Measures(filepath.Join("/proc", pid.String(), "status"))

	ppid, _ := strconv.Atoi(fields[3])
	pgid, _ := strconv.Atoi(fields[4])
	tgid, _ := strconv.Atoi(fields[7])
	tty, _ := strconv.Atoi(fields[6])
	priority, _ := strconv.Atoi(fields[17])
	nice, _ := strconv.Atoi(fields[18])
	threads, _ := strconv.Atoi(fields[19])
	start, _ := strconv.Atoi(fields[21])
	uid, _ := strconv.Atoi(m["Uid"])
	gid, _ := strconv.Atoi(m["Gid"])

	user, _ := strconv.ParseUint(fields[13], 10, 64)
	system, _ := strconv.ParseUint(fields[14], 10, 64)

	size, _ := strconv.Atoi(m["VmSize"])
	resident, _ := strconv.Atoi(m["VmRSS"])
	rssFile, _ := strconv.Atoi(m["RssFile"])
	rssShmem, _ := strconv.Atoi(m["RssShmem"])
	vmMax, _ := strconv.Atoi(m["VmPeak"])
	rssMax, _ := strconv.Atoi(m["VmHWM"])
	minorFaults, _ := strconv.Atoi(fields[9])
	majorFaults, _ := strconv.Atoi(fields[11])

	voluntaryContextSwitches, _ := strconv.Atoi(m["voluntary_ctxt_switches"])
	nonVoluntaryContextSwitches, _ := strconv.Atoi(m["nonvoluntary_ctxt_switches"])

	return Id{
			ppid:      Pid(ppid),
			Name:      fields[1][1 : len(fields[1])-1],
			Pid:       pid,
			Starttime: core.Boottime.Add(time.Duration(start) * factor),
		},
		Properties{
			Ppid:        Pid(ppid),
			Pgid:        pgid,
			Tgid:        tgid,
			Tty:         fmt.Sprintf("%#.8X", tty),
			UID:         uid,
			GID:         gid,
			Username:    core.Username(uid),
			Groupname:   core.Groupname(gid),
			Status:      status[fields[2][0]],
			Nice:        nice,
			CommandLine: pid.commandLine(),
			Directories: pid.directories(),
		},
		Metrics{
			Priority:                    priority,
			Threads:                     threads,
			User:                        time.Duration(user),
			System:                      time.Duration(system),
			Total:                       time.Duration(user + system),
			Size:                        size * 1024,
			Resident:                    resident * 1024,
			Share:                       (rssFile + rssShmem) * 1024,
			VirtualMemoryMax:            vmMax * 1024,
			ResidentMemoryMax:           rssMax * 1024,
			PageFaults:                  minorFaults + majorFaults,
			MinorFaults:                 minorFaults,
			MajorFaults:                 majorFaults,
			VoluntaryContextSwitches:    voluntaryContextSwitches,
			NonVoluntaryContextSwitches: nonVoluntaryContextSwitches,
			ContextSwitches:             voluntaryContextSwitches + nonVoluntaryContextSwitches,
			Io:                          pid.io(),
		}
}

// io captures process I/O counts.
func (pid Pid) io() Io {
	i := Io{}
	m, err := core.Measures(filepath.Join("/proc", pid.String(), "io"))
	if err != nil {
		core.LogError(err)
		return i
	}

	i.ReadRequested, _ = strconv.Atoi(m["rchar"])
	i.WriteRequested, _ = strconv.Atoi(m["wchar"])
	i.ReadActual, _ = strconv.Atoi(m["read_bytes"])
	i.WriteActual, _ = strconv.Atoi(m["write_bytes"])
	i.ReadOperations, _ = strconv.Atoi(m["syscr"])
	i.WriteOperations, _ = strconv.Atoi(m["syscw"])

	return i
}

// commandLine retrieves process command, arguments, and environment.
func (pid Pid) commandLine() CommandLine {
	clLock.RLock()
	cl, ok := clMap[pid]
	clLock.RUnlock()
	if ok {
		return cl
	}

	cl.Executable, _ = os.Readlink(filepath.Join("/proc", pid.String(), "exe"))

	if arg, err := os.ReadFile(filepath.Join("/proc", pid.String(), "cmdline")); err == nil {
		cl.Args = strings.Split(string(arg[:len(arg)-2]), "\000")
		cl.Args = cl.Args[1:]
	}

	if env, err := os.ReadFile(filepath.Join("/proc", pid.String(), "environ")); err == nil {
		cl.Envs = strings.Split(string(env), "\000")
	}

	clLock.Lock()
	clMap[pid] = cl
	clLock.Unlock()

	return cl
}

// directories captures process directories.
func (pid Pid) directories() Directories {
	d := Directories{}
	d.Cwd, _ = os.Readlink(filepath.Join("/proc", pid.String(), "cwd"))
	d.Root, _ = os.Readlink(filepath.Join("/proc", pid.String(), "root"))
	return d
}

// getPids gets the list of active processes by pid.
func getPids() ([]Pid, error) {
	dir, err := os.Open("/proc")
	if err != nil {
		return nil, core.Error("/proc", err)
	}
	ns, err := dir.Readdirnames(0)
	dir.Close()
	if err != nil {
		return nil, core.Error("/proc", err)
	}

	pids := make([]Pid, 0, len(ns))
	i := 0
	for _, n := range ns {
		if pid, err := strconv.Atoi(n); err == nil {
			pids[i] = Pid(pid)
			i++
		}
	}

	return pids, nil
}
