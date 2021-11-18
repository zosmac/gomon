// Copyright © 2021 The Gomon Project.

package core

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"time"
)

// FromCString interprets a null terminated C char array as a GO string.
func FromCString(p []int8) string {
	var s string
	for _, c := range p {
		if c == 0 {
			break
		}
		s += string(byte(c))
	}
	return s
}

// FdPath gets the path for an open file descriptor.
func FdPath(fd int) (string, error) {
	pid := os.Getpid()
	return os.Readlink("/proc/" + strconv.Itoa(pid) + "/fd/" + strconv.Itoa(fd))
}

// MountMap builds a map of mount points to file systems.
func MountMap() (map[string]string, error) {
	f, err := os.Open("/etc/mtab")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	m := map[string]string{"/": ""} // have "/" at a minimum
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		m[f[1]] = f[0]
	}
	return m, nil
}

// boottime gets the system boot time.
func boottime() time.Time {
	f, err := os.Open("/proc/stat")
	if err != nil {
		LogError(NewError("/proc/stat open", err))
		return time.Time{}
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		l := sc.Text()
		kv := strings.SplitN(l, " ", 2)
		switch kv[0] {
		case "btime":
			sec, err := strconv.Atoi(kv[1])
			if err != nil {
				LogError(NewError("/proc/stat btime", err))
				return time.Time{}
			}
			return time.Unix(int64(sec), 0)
		}
	}

	LogError(NewError("/proc/stat btime", sc.Err()))
	return time.Time{}
}

// Measures reads a /proc filesystem file and produces a map of name:value pairs.
func Measures(filename string) (map[string]string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	m := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		s := strings.SplitN(sc.Text(), ":", 2)
		if len(s) == 2 {
			k := s[0]
			v := strings.Fields(s[1])
			if len(v) > 0 {
				m[k] = v[0]
			}
		}
	}

	return m, nil
}
