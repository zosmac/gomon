// Copyright © 2021-2023 The Gomon Project.

package file

// import (
// 	"C"
// )

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"github.com/zosmac/gocore"
)

var (
	// localTypes identifies the names for local filesystem types.
	localTypes = map[string]struct{}{"rootfs": {}, "aufs": {}}

	// mountTypes identifies types of mount points.
	mountTypes, _ = gocore.MountMap()
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
		return nil, gocore.Error("inotify_init", err)
	}

	// IN_DELETE_SELF evidently not sent to root directory, therefore watch its parent directory for IN_DELETE.
	wd, err := syscall.InotifyAddWatch(
		fd,
		filepath.Join(directory, ".."),
		syscall.IN_ONLYDIR|syscall.IN_MOVED_FROM|syscall.IN_DELETE,
	)
	if err != nil {
		return nil, gocore.Error("inotify_add_watch", err)
	}

	return &handle{
		fd:      fd,
		wd:      wd,
		watches: map[int]string{},
	}, nil
}

// close OS resources.
func (h *handle) close() {
	syscall.InotifyRmWatch(h.fd, uint32(h.wd))
	syscall.Close(h.fd)
}

// observe inotify events and notify observer's callbacks.
func observe() error {
	go func() {
		for {
			events := make([]byte, 16384)
			n, err := syscall.Read(obs.fd, events)
			if err != nil {
				if !errors.Is(err, syscall.EBADF) {
					gocore.Error("Read", err).Err()
				}
				return
			}

			var event *syscall.InotifyEvent
			for i := 0; i < n; i += syscall.SizeofInotifyEvent + int(event.Len) {
				event = (*syscall.InotifyEvent)(unsafe.Pointer(&events[i]))
				id := strconv.Itoa(int(event.Cookie))

				if event.Mask&syscall.IN_IGNORED != 0 {
					continue
				}

				var abs string
				if event.Len > 0 {
					abs = gocore.GoStringN((*byte)(unsafe.Pointer(&event.Name)), event.Len)
				}

				// IN_DELETE_SELF evidently not sent to root, therefore test if its parent received IN_DELETE
				if int(event.Wd) == obs.wd &&
					abs == obs.root &&
					event.Mask&(syscall.IN_ISDIR|syscall.IN_DELETE) == (syscall.IN_ISDIR|syscall.IN_DELETE) {
					return
				}

				f, ok := obs.watched[obs.watches[int(event.Wd)]]
				if ok {
					abs = f.abs
				} else {
					syscall.InotifyRmWatch(obs.fd, uint32(event.Wd))
					continue
				}

				if event.Mask&(syscall.IN_CREATE|syscall.IN_MOVED_TO) != 0 {
					if info, err := os.Stat(abs); err != nil {
						continue
					} else if info.IsDir() {
						rel, _ := filepath.Rel(obs.root, abs)
						watchDir(rel, id)
					} else {
						add(abs, false)
						if info.Size() > 0 {
							event.Mask |= syscall.IN_MODIFY
						}
						notify(fileCreate, id, f.abs, "")
					}
				}

				if event.Mask&syscall.IN_MODIFY != 0 {
					notify(fileUpdate, id, f.abs, "")
				}

				if event.Mask&(syscall.IN_DELETE|syscall.IN_MOVED_FROM) != 0 {
					remove(f, id)
				}
			}
		}
	}()

	return nil
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
		return gocore.Error("inotify_add_watch", err)
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
