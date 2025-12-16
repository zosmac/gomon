// Copyright © 2021-2023 The Gomon Project.

package file

import (
	"os"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"github.com/zosmac/gocore"

	"golang.org/x/sys/windows"
)

// RetryableError reports if error may be temporary.
func (f file) RetryableError(err error) bool {
	for wait := 250 * time.Millisecond; wait < 4*time.Second && tempError(err); wait *= 2 {
		<-time.After(wait)
		file, err := os.Open(f.abs)
		if err == nil {
			file.Close()
			return true
		}
	}
	return false
}

const errorSharingViolation = syscall.Errno(32)

// tempError identifies a temporary error.
func tempError(err error) bool {
	return err.(*os.PathError).Err.(syscall.Errno) == errorSharingViolation
}

// handle defines host specific observer properties.
type handle struct {
	windows.Handle
}

// open obtains a directory handle for observer.
func open(directory string) (*handle, error) {
	cwd, _ := windows.UTF16PtrFromString(directory)
	h, err := windows.CreateFile(
		cwd,
		windows.FILE_LIST_DIRECTORY,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		return nil, gocore.Error("CreateFile", err)
	}

	return &handle{h}, nil
}

// close OS resources.
func (h *handle) close() {
	windows.CloseHandle(h.Handle)
}

// observe events and notify observer's callbacks.
func observe() error {
	go func() {
		runtime.LockOSThread() // tie this goroutine to an OS thread
		defer runtime.UnlockOSThread()

		// Because of file caching, windows.FILE_NOTIFY_CHANGE_SIZE may not be timely.
		// So poll files with GetFileAttributesEx to trigger writes to disk.
		// See the poll() function below.
		go poll()

		filter := uint32(windows.FILE_NOTIFY_CHANGE_LAST_WRITE |
			windows.FILE_NOTIFY_CHANGE_SIZE |
			windows.FILE_NOTIFY_CHANGE_ATTRIBUTES |
			windows.FILE_NOTIFY_CHANGE_FILE_NAME |
			windows.FILE_NOTIFY_CHANGE_DIR_NAME,
		)

		for {
			events := make([]byte, 65536)
			var n uint32
			if err := windows.ReadDirectoryChanges(
				obs.Handle,
				&events[0],
				uint32(len(events)),
				true,
				filter,
				&n,
				nil,
				0,
			); err != nil {
				windows.Close(obs.Handle)
				obs.Handle = windows.InvalidHandle
				gocore.Error("ReadDirectoryChanges", err).Err()
				return
			}

			var event *windows.FileNotifyInformation
			for i := 0; i < int(n); i += int(event.NextEntryOffset) {
				event = (*windows.FileNotifyInformation)(unsafe.Pointer(&events[i]))
				name := windows.UTF16ToString((*[1024]uint16)(unsafe.Pointer(&event.FileName))[:event.FileNameLength/2])

				switch event.Action {
				case windows.FILE_ACTION_ADDED, windows.FILE_ACTION_RENAMED_NEW_NAME:
					if info, err := os.Stat(name); err != nil {
						continue
					} else if info.IsDir() {
						watchDir(name, 0)
					} else {
						if f, err := add(name, false); err == nil {
							notify(fileCreate, 0, f.abs, "")
						}
					}
				case windows.FILE_ACTION_MODIFIED:
					notify(fileUpdate, 0, obs.watched[name].abs, "")
				case windows.FILE_ACTION_REMOVED, windows.FILE_ACTION_RENAMED_OLD_NAME:
					remove(obs.watched[name], 0)
				}

				if event.NextEntryOffset == 0 {
					break
				}
			}
		}
	}()

	return nil
}

// poll invokes os.Stat to flush cached files to disk. On Windows, files are not
// flushed automatically and may be cached for a substantial amount of time. The
// ReadDirectoryChangesW API doc acknowledges this behavior. Fortunately, periodically
// polling each file with os.Stat will report if any files have been updated. The
// Go os.Stat function on Windows calls GetFileAttributesEx.
func poll() {
	const poll = 10 * time.Second
	const poll80 = int(poll) * 4 / 5 // 80% of poll interval
	ticker := time.NewTicker(poll)

	for range ticker.C {
		var fs []file
		for _, f := range obs.watched {
			fs = append(fs, f)
		}
		if len(fs) == 0 {
			continue
		}

		d := poll80 / len(fs)
		t := time.NewTicker(time.Duration(d))

		// distribute stat() calls over 80% of poll interval, rather than all at once
		i := 0
		for range t.C {
			os.Stat(fs[i].abs)
			i++
			if i >= len(fs) {
				t.Stop()
				break
			}
		}
	}
}

// addDir adds host specific handling for a directory.
func addDir(abs string) error {
	return nil
}

// removeDir removes host specific handling for a directory.
func removeDir(abs string) {
}
