// Copyright © 2021 The Gomon Project.

package logs

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/zosmac/gomon/core"
	"github.com/zosmac/gomon/message"
)

var (
	// inotify descriptor
	nd int

	// watched maps process pids to watch descriptors of open log files.
	watched = map[int]map[int]*os.File{}
	wLock   sync.Mutex

	// regex for parsing log records.
	regex = regexp.MustCompile(
		`(?m:^ ?\[? ?(?P<timestamp>(?:\d{4}[\/-]\d\d[\/-]\d\d[ T]\d\d:\d\d:\d\d)(?:\.(?:\d\d\d\d\d\d|\d\d\d)|))` +
			`(?:(?P<utc>Z)| ?(?P<tzoffset>[+-]\d\d:?\d\d)|)(?:(?P<timezone> [A-Z]{3})|)` +
			`(?:(?: \[|: pid )(?P<pid>\d+)\]?|)` +
			`(?: \[?(?P<level>err|log|[A-Za-z]{4,5}[1-9]?)\]?|)?[ :\]]+` +
			`(?P<message>.*)$|\z)`)

	// groups maps capture group names to indices.
	groups = func() map[string]int {
		g := map[string]int{}
		for _, name := range regex.SubexpNames() {
			g[string(name)] = regex.SubexpIndex(name)
		}
		return g
	}()
)

const (
	// log record regular expressions capture group names.
	groupTimestamp = "timestamp"
	groupUtc       = "utc"
	groupTzoffset  = "tzoffset"
	groupTimezone  = "timezone"
	groupLevel     = "level"
	groupPid       = "pid"
	groupMessage   = "message"
)

// open obtains a watch handle for observer
func open() error {
	var err error
	if flags.logDirectory, err = filepath.Abs(flags.logDirectory); err != nil {
		return core.Error("Abs", err)
	}
	if flags.logDirectory, err = filepath.EvalSymlinks(flags.logDirectory); err != nil {
		return core.Error("EvalSymlinks", err)
	}

	core.LogInfo(
		fmt.Errorf(
			"watching logs in directory %s, include pattern: %s, exclude pattern: %s",
			flags.logDirectory,
			flags.logRegex,
			flags.logRegexExclude,
		),
	)

	nd, err = syscall.InotifyInit()
	if err != nil {
		return core.Error("inotify_init", err)
	}

	return nil
}

// close OS resources.
func close() error {
	for _, wds := range watched {
		for wd, l := range wds {
			l.Close()
			syscall.InotifyRmWatch(nd, uint32(wd))
		}
	}
	watched = nil
	syscall.Close(nd)
	nd = -1
	return nil
}

// observe inotify events and notify observer's callbacks
func observe() {
	defer close()

	for {
		events := make([]byte, 16384)
		n, err := syscall.Read(nd, events)
		if err != nil {
			errorChan <- core.Error("read", err)
			return
		}

		ready := map[int]*os.File{}
		var event *syscall.InotifyEvent
		for i := 0; i < n; i += syscall.SizeofInotifyEvent + int(event.Len) {
			event = (*syscall.InotifyEvent)(unsafe.Pointer(&events[i]))

			if event.Mask&syscall.IN_IGNORED != 0 {
				continue
			}

			// verify log file still being watched
			var l *os.File
			var ok bool
			for _, wds := range watched {
				if l, ok = wds[int(event.Wd)]; ok {
					break
				}
			}
			if !ok {
				delete(ready, int(event.Wd))
				syscall.InotifyRmWatch(nd, uint32(event.Wd))
				continue
			}

			if event.Mask&syscall.IN_MODIFY != 0 {
				ready[int(event.Wd)] = l
			}
		}

		report(ready)
	}
}

func report(ready map[int]*os.File) {
	for _, l := range ready {
		if info, err := l.Stat(); err == nil {
			current, _ := l.Seek(0, io.SeekCurrent)
			offset := info.Size() - current
			if offset <= 0 {
				l.Seek(int64(offset), io.SeekCurrent)
				continue
			}
			buf := make([]byte, offset)
			n, _ := l.Read(buf)
			matches := regex.FindAllStringSubmatch(string(buf[:n]), -1)
			for _, match := range matches {
				if len(match) == 0 || match[0] == "" {
					continue
				}

				level := levelMap[strings.ToLower(match[groups[groupLevel]])]
				if !logLevels.IsValid(string(level)) || logLevels.Index(level) < logLevels.Index(flags.logLevel) {
					continue
				}

				var timestamp time.Time
				t := match[groups[groupTimestamp]]
				t = strings.Replace(t, "/", "-", 2) // replace '/' with '-' in date
				t = strings.Replace(t, "T", " ", 1) // replace 'T' with ' ' between date and time

				if match[groups[groupUtc]] == "Z" || match[groups[groupTzoffset]] != "" {
					t += match[groups[groupUtc]] + match[groups[groupTzoffset]]
					timestamp, _ = time.Parse("2006-01-02 15:04:05Z07:00", t)
				} else if match[groups[groupTimezone]] != "" {
					t += match[groups[groupTimezone]]
					timestamp, _ = time.Parse("2006-01-02 15:04:05 MST", t)
				}

				pid, _ := strconv.Atoi(match[groups[groupPid]])
				messageChan <- &observation{
					Header: message.Observation(timestamp, level),
					Id: Id{
						Pid: pid,
					},
					Message: match[groups[groupMessage]],
				}
			}
		}
	}
}

// Watch adds a process' logs to watch to the observer.
func Watch(name string, pid int) {
	for _, l := range watched[pid] {
		if name == l.Name() { // already watching
			continue
		}
	}
	if err := filter(name); err != nil {
		return
	}

	l, err := os.Open(name)
	if err != nil {
		return
	}
	l.Seek(0, io.SeekEnd)

	wd, err := syscall.InotifyAddWatch(nd, name, uint32(syscall.IN_MODIFY))
	if err != nil {
		l.Close()
		return
	}

	wLock.Lock()
	if _, ok := watched[pid]; !ok {
		watched[pid] = map[int]*os.File{}
	}
	watched[pid][wd] = l
	wLock.Unlock()
}

// Remove exited processes' logs from observation.
func Remove(pids []int) {
	wLock.Lock()
	defer wLock.Unlock()
	for _, pid := range pids {
		if pid == os.Getpid() {
			continue
		}
		for wd, l := range watched[pid] {
			l.Close()
			syscall.InotifyRmWatch(nd, uint32(wd))
		}
		delete(watched, pid)
	}
}

// filter determines whether a log file should be watched.
func filter(abs string) error {
	if path, err := filepath.Rel(flags.logDirectory, abs); err != nil || strings.HasPrefix(path, "..") {
		return fmt.Errorf("%s not in %s path", abs, flags.logDirectory)
	}

	if !flags.logRegex.MatchString(abs) {
		return fmt.Errorf("%s no match %s", abs, flags.logRegex.String())
	}

	if flags.logRegexExclude.MatchString(abs) {
		return fmt.Errorf("%s excluded %s", abs, flags.logRegexExclude.String())
	}

	return nil
}
