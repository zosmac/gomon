// Copyright © 2021-2023 The Gomon Project.

package logs

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"syscall"
	"unsafe"

	"github.com/zosmac/gocore"
)

var (
	// inotify descriptor
	nd int

	// watched maps process pids to watch descriptors of open log files.
	watched = map[int]map[int]*os.File{}
	wLock   sync.Mutex

	// regex for parsing log records.
	regex = regexp.MustCompile(
		`^ ?\[? ?(?P<timestamp>(?:\d{4}[\/-]\d\d[\/-]\d\d[ T]\d\d:\d\d:\d\d)(?:\.(?:\d\d\d\d\d\d|\d\d\d)|)` +
			`(?:(?P<utc>Z)| ?(?P<tzoffset>[+-]\d\d:?\d\d)|)(?:(?P<timezone> [A-Z]{3})|))` +
			`(?:(?: \[|: pid )(?P<pid>\d+)\]?|)` +
			`(?: \[?(?P<level>err|log|[A-Za-z]{4,5}[1-9]?)\]?|)?[ :\]]+` +
			`(?P<message>.*)$`)

	// groups maps capture group names to indices.
	groups = func() map[string]int {
		g := map[string]int{}
		for _, name := range regex.SubexpNames() {
			g[string(name)] = regex.SubexpIndex(name)
		}
		return g
	}()
)

// open obtains a watch handle for observer.
func open() error {
	var err error
	if Flags.logDirectory, err = filepath.Abs(Flags.logDirectory); err != nil {
		return gocore.Error("Abs", err)
	}
	if Flags.logDirectory, err = filepath.EvalSymlinks(Flags.logDirectory); err != nil {
		return gocore.Error("EvalSymlinks", err)
	}

	gocore.Error("log observer", nil, map[string]string{
		"directory":       Flags.logDirectory,
		"include_pattern": Flags.logRegex.String(),
		"exclude_pattern": Flags.logRegexExclude.String(),
	}).Info()

	nd, err = syscall.InotifyInit()
	if err != nil {
		return gocore.Error("inotify_init", err)
	}

	return nil
}

// close OS resources.
func close() {
	for _, wds := range watched {
		for wd, l := range wds {
			l.Close()
			syscall.InotifyRmWatch(nd, uint32(wd))
		}
	}
	watched = nil
	syscall.Close(nd)
	nd = -1
}

// observe inotify events and notify observer's callbacks.
func observe(_ context.Context) error {
	go func() {
		for {
			events := make([]byte, 16384)
			n, err := syscall.Read(nd, events)
			if err != nil {
				if !errors.Is(err, syscall.EBADF) {
					gocore.Error("Read", err).Err()
				}
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
	}()

	return nil
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

			sc := bufio.NewScanner(l)

			go parseLog(sc, regex, "")
		}
	}
}

// Watch adds a process' logs to watch to the observer.
func Watch(name string, pid int) {
	for _, l := range watched[pid] {
		if name == l.Name() { // already watching
			return
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
	if _, err := gocore.Subdir(Flags.logDirectory, abs); err != nil {
		return fmt.Errorf("%s not in %s path", abs, Flags.logDirectory)
	}

	if !Flags.logRegex.MatchString(abs) {
		return fmt.Errorf("%s no match %s", abs, Flags.logRegex.String())
	}

	if Flags.logRegexExclude.MatchString(abs) {
		return fmt.Errorf("%s excluded %s", abs, Flags.logRegexExclude.String())
	}

	return nil
}
