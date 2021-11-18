// Copyright © 2021 The Gomon Project.

package file

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"github.com/zosmac/gomon/core"
)

var (
	// localTypes identifies the names for local filesystem types.
	localTypes = map[string]struct{}{"rootfs": {}, "aufs": {}}

	// mountTypes identifies types of mount points.
	mountTypes = map[string]string{}
)

// init initializes the local filesystem types and the mount types.
func init() {
	if f, err := os.Open("/proc/filesystems"); err == nil {
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			f := strings.Split(sc.Text(), "\t")
			if f[0] != "nodev" {
				localTypes[f[1]] = struct{}{}
			}
		}
	}

	// build map of mounts and their types
	if f, err := os.Open("/etc/mtab"); err == nil {
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			f := strings.Fields(sc.Text())
			mountTypes[f[1]] = f[2]
		}
	}
}

// handle defines host specific observer properties.
type handle struct {
	fd      int            // inotify descriptor
	wd      int            // watch descriptor for parent directory
	watches map[int]string // map of directory watch descriptors to directory paths
}

// open obtains a directory handle for observer.
func open(directory string) (*handle, error) {
	fd, err := syscall.InotifyInit()
	if err != nil {
		return nil, core.NewError("inotify_init", err)
	}

	// IN_DELETE_SELF evidently not sent to root directory, therefore watch its parent directory for IN_DELETE.
	wd, err := syscall.InotifyAddWatch(fd, filepath.Join(directory, ".."),
		syscall.IN_ONLYDIR|syscall.IN_MOVED_FROM|syscall.IN_DELETE)
	if err != nil {
		return nil, core.NewError("inotify_add_watch", err)
	}

	return &handle{
		fd:      fd,
		wd:      wd,
		watches: map[int]string{},
	}, nil
}

// close OS resources.
func (h *handle) close() error {
	syscall.InotifyRmWatch(h.fd, uint32(h.wd))
	syscall.Close(h.fd)
	return nil
}

// listen for inotify events and notify observer's callbacks.
func listen() {
	defer obs.close()

	for {
		events := make([]byte, 16384)
		n, err := syscall.Read(obs.fd, events)
		if err != nil {
			errorChan <- core.NewError("read", err)
			return
		}

		var event *syscall.InotifyEvent
		for i := 0; i < n; i += syscall.SizeofInotifyEvent + int(event.Len) {
			event = (*syscall.InotifyEvent)(unsafe.Pointer(&events[i]))

			if event.Mask&syscall.IN_IGNORED != 0 {
				continue
			}

			var abs string
			if event.Len > 0 {
				arr := (*[1024]byte)(unsafe.Add(unsafe.Pointer(event), syscall.SizeofInotifyEvent))
				abs = string(abs[:bytes.IndexByte(arr[:], 0)])
			}

			// IN_DELETE_SELF evidently not sent to root, therefore test if its parent received IN_DELETE
			if int(event.Wd) == obs.wd &&
				abs == obs.root &&
				event.Mask&(syscall.IN_ISDIR|syscall.IN_DELETE) == (syscall.IN_ISDIR|syscall.IN_DELETE) {
				return
			}

			f, ok := obs.watched[obs.watches[int(event.Wd)]]
			if ok {
				abs = f.name
			} else {
				syscall.InotifyRmWatch(obs.fd, uint32(event.Wd))
				continue
			}

			if event.Mask&(syscall.IN_CREATE|syscall.IN_MOVED_TO) != 0 {
				if info, err := os.Stat(abs); err != nil {
					continue
				} else if info.IsDir() {
					rel, _ := filepath.Rel(obs.root, abs)
					watchDir(rel)
				} else {
					add(abs, false)
					if info.Size() > 0 {
						event.Mask |= syscall.IN_MODIFY
					}
					notify(fileCreate, f, "")
				}
			}

			if event.Mask&syscall.IN_MODIFY != 0 {
				notify(fileUpdate, f, "")
			}

			if event.Mask&(syscall.IN_DELETE|syscall.IN_MOVED_FROM) != 0 {
				remove(f)
			}
		}
	}
}

// addDir adds host specific handling for a directory.
// Refer to http://man7.org/linux/man-pages/man7/inotify.7.html
func addDir(abs string) error {
	mask := uint32(syscall.IN_ONLYDIR |
		syscall.IN_CREATE |
		syscall.IN_MOVE |
		syscall.IN_MODIFY |
		syscall.IN_DELETE |
		syscall.IN_EXCL_UNLINK,
	)
	wd, err := syscall.InotifyAddWatch(obs.fd, abs, mask)
	if err != nil {
		// if ENOSPC, archive unused files or increase fs.inotify.max_user_watches
		return core.NewError("inotify_add_watch", err)
	}
	obs.watches[wd] = abs
	return nil
}

// removeDir removes host specific handling for a directory.
func removeDir(abs string) {
	for wd, n := range obs.watches {
		if n == abs {
			syscall.InotifyRmWatch(obs.fd, uint32(wd))
			delete(obs.watches, wd)
			break
		}
	}
}
