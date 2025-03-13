// Copyright © 2021-2023 The Gomon Project.

package system

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/zosmac/gocore"
)

var (
	// uname provides the name of the Operating System.
	uname = func() string {
		var uname syscall.Utsname
		if syscall.Uname(&uname) != nil {
			return ""
		}

		ss := []string{
			gocore.GoStringN(&uname.Sysname[0], len(uname.Sysname)),
			gocore.GoStringN(&uname.Nodename[0], len(uname.Nodename)),
			gocore.GoStringN(&uname.Release[0], len(uname.Release)),
			gocore.GoStringN(&uname.Version[0], len(uname.Version)),
			gocore.GoStringN(&uname.Machine[0], len(uname.Machine)),
			gocore.GoStringN(&uname.Domainname[0], len(uname.Domainname)),
		}

		return strings.Join(ss, " ")
	}()

	// factor is the system units for CPU time (i.e. "ticks" or "jiffies").
	factor = 10000 * time.Microsecond
)

// loadAverage gets the system load averages.
func loadAverage() LoadAverage {
	buf, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		gocore.Error("/proc/loadavg", err).Err()
		return LoadAverage{}
	}
	f := strings.Fields(string(buf))
	one, _ := strconv.ParseFloat(f[0], 64)
	five, _ := strconv.ParseFloat(f[1], 64)
	fifteen, _ := strconv.ParseFloat(f[2], 64)
	return LoadAverage{
		OneMinute:     one,
		FiveMinute:    five,
		FifteenMinute: fifteen,
	}
}

// contextSwitches queries count of system context switches.
func contextSwitches() int {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		l := sc.Text()
		k, v, _ := strings.Cut(l, " ")
		switch k {
		case "ctxt":
			s, err := strconv.Atoi(v)
			if err != nil {
				return 0
			}
			return s
		}
	}

	return 0
}

// rlimits gets system resource limits.
func rlimits() Rlimits {
	l := Rlimits{}
	var limit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_AS, &limit); err == nil {
		l.MemoryPerUser = int(limit.Max)
	}
	if err := syscall.Getrlimit(6 /*syscall.RLIMIT_NPROC*/, &limit); err == nil {
		l.ProcessesPerUser = int(limit.Max)
	}
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &limit); err == nil {
		l.OpenFilesPerProcess = int(limit.Max)
	}
	if buf, err := os.ReadFile("/proc/sys/fs/file-max"); err == nil {
		l.OpenFilesMaximum, _ = strconv.Atoi(string(buf))
	}

	return l
}

// cpu captures CPU metrics for system.
func cpu() CPU {
	f, err := os.Open("/proc/stat")
	if err != nil {
		gocore.Error("/proc/stat open", err).Err()
		return CPU{}
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		l := sc.Text()
		k, v, _ := strings.Cut(l, " ")
		switch k {
		case "cpu":
			return scale(v)
		}
	}

	gocore.Error("/proc/stat cpu", sc.Err()).Err()
	return CPU{}
}

// cpus captures individual CPU metrics.
func cpus() []CPU {
	f, err := os.Open("/proc/stat")
	if err != nil {
		gocore.Error("/proc/stat open", err).Err()
		return nil
	}
	defer f.Close()

	var cpus []CPU
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		l := sc.Text()
		if len(l) > 3 && l[:3] == "cpu" && l[3] != ' ' {
			_, v, _ := strings.Cut(l[3:], " ")
			cpus = append(cpus, scale(v))
		}
	}
	if len(cpus) == 0 && sc.Err() != nil {
		gocore.Error("/proc/stat cpu", sc.Err()).Err()
		return nil
	}
	return cpus
}

// scale converts cpu times to nanoseconds.
func scale(stat string) CPU {
	flds := strings.Fields(stat)
	user, _ := strconv.Atoi(flds[1])
	nice, _ := strconv.Atoi(flds[2])
	system, _ := strconv.Atoi(flds[3])
	idle, _ := strconv.Atoi(flds[4])
	iowait, _ := strconv.Atoi(flds[5])
	irq, _ := strconv.Atoi(flds[6])
	softIrq, _ := strconv.Atoi(flds[7])
	stolen, _ := strconv.Atoi(flds[8])

	c := CPU{
		User:    time.Duration(user) * factor,
		System:  time.Duration(system) * factor,
		Idle:    time.Duration(idle) * factor,
		Nice:    time.Duration(nice) * factor,
		IoWait:  time.Duration(iowait) * factor,
		Irq:     time.Duration(irq) * factor,
		SoftIrq: time.Duration(softIrq) * factor,
		Stolen:  time.Duration(stolen) * factor,
	}
	c.Total = c.User + c.System + c.Idle + c.Nice + c.IoWait + c.Irq + c.SoftIrq + c.Stolen
	return c
}

// memory captures system's memory and swap metrics.
func memory() (Memory, Swap) {
	i, err := gocore.Measures("/proc/meminfo")
	if err != nil {
		gocore.Error("/proc/meminfo", err).Err()
	}

	total, _ := strconv.Atoi(i["MemTotal"])
	free, _ := strconv.Atoi(i["MemFree"])
	buffers, _ := strconv.Atoi(i["Buffers"])
	cached, _ := strconv.Atoi(i["Cached"])
	freeActual := free + buffers + cached
	if available, ok := i["MemAvailable"]; ok { // kernel 3.14+
		freeActual, _ = strconv.Atoi(available)
	}
	swapTotal, _ := strconv.Atoi(i["SwapTotal"])
	swapFree, _ := strconv.Atoi(i["SwapFree"])

	return Memory{
			Total:      total,
			Free:       free,
			Used:       total - free,
			FreeActual: freeActual,
			UsedActual: total - freeActual,
		},
		Swap{
			Total: swapTotal,
			Free:  swapFree,
			Used:  swapTotal - swapFree,
		}
}
