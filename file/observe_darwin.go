// Copyright © 2021 The Gomon Project.

package file

/*
#cgo CFLAGS: -x objective-c -std=gnu11 -fobjc-arc
#cgo LDFLAGS: -framework CoreServices
#import <CoreServices/CoreServices.h>

// callback handles events sent on stream
extern void callback(ConstFSEventStreamRef, void *, size_t, char **, FSEventStreamEventFlags *, FSEventStreamEventId *);
*/
import "C"

import (
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"unsafe"

	"github.com/zosmac/gomon/core"
)

// handle defines host specific observer properties.
type handle struct {
}

// open obtains a directory handle for observer.
func open(directory string) (*handle, error) {
	return &handle{}, nil
}

// close OS resources.
func (h *handle) close() {
}

// listen for events and notify observer's callbacks.
func listen() {
	defer obs.close()
	runtime.LockOSThread() // tie this goroutine to an OS thread
	defer runtime.UnlockOSThread()

	runloop := C.CFRunLoopGetCurrent()
	defer func() {
		C.CFRunLoopStop(runloop)
	}()

	cname := core.CreateCFString(flags.fileDirectory + "\x00")
	defer C.CFRelease(C.CFTypeRef(cname))
	context := C.malloc(C.sizeof_struct_FSEventStreamContext)
	defer C.free(context)
	C.memset(context, 0, C.sizeof_struct_FSEventStreamContext)

	stream := C.FSEventStreamCreate(
		0,
		(*[0]byte)(C.callback),
		(*C.FSEventStreamContext)(context),
		C.CFArrayCreate(0, &cname, 1, nil),
		C.kFSEventStreamEventIdSinceNow,
		C.CFTimeInterval(1.0),
		C.kFSEventStreamCreateFlagFileEvents|C.kFSEventStreamCreateFlagWatchRoot, // |C.kFSEventStreamCreateFlagNoDefer,
	)
	defer func() {
		C.FSEventStreamStop(stream)
		C.FSEventStreamInvalidate(stream)
		C.FSEventStreamRelease(stream)
	}()

	// NOTE: KEEP THIS AS A USEFUL EXAMPLE
	// paths := C.FSEventStreamCopyPathsBeingWatched(stream)
	// defer C.CFRelease(C.CFTypeRef(paths))
	// var buf [1024]C.char
	// C.CFStringGetCString(
	// 	C.CFStringRef(C.CFArrayGetValueAtIndex(paths, 0)),
	// 	&buf[0],
	// 	C.CFIndex(len(buf)),
	// 	C.kCFStringEncodingUTF8,
	// )
	// fmt.Fprintf(os.Stderr, "top level path is %s\n", C.GoString(&buf[0]))

	C.FSEventStreamScheduleWithRunLoop(stream, runloop, C.kCFRunLoopDefaultMode)
	C.FSEventStreamStart(stream)
	C.CFRunLoopRun() // start watching
}

// callback handles events sent on stream.
//export callback
func callback(stream C.ConstFSEventStreamRef, _ unsafe.Pointer, count C.size_t, paths **C.char, flags *C.FSEventStreamEventFlags, ids *C.FSEventStreamEventId) {
	var oldname string
	var oldisdir bool

	for i := C.size_t(0); i < count; i++ {
		abs := C.GoString(*paths)
		rel, err := filepath.Rel(obs.root, abs)
		flag := *flags
		// prepare for next
		paths = (**C.char)(unsafe.Add(unsafe.Pointer(paths), unsafe.Sizeof(*paths)))
		flags = (*C.FSEventStreamEventFlags)(unsafe.Add(unsafe.Pointer(flags), C.sizeof_FSEventStreamEventFlags))
		ids = (*C.FSEventStreamEventId)(unsafe.Add(unsafe.Pointer(ids), C.sizeof_FSEventStreamEventId))

		if err != nil {
			continue
		}

		// do we care about symlink changes?
		isDir := flag&C.kFSEventStreamEventFlagItemIsDir == C.kFSEventStreamEventFlagItemIsDir
		isFile := flag&C.kFSEventStreamEventFlagItemIsFile == C.kFSEventStreamEventFlagItemIsFile
		if !isDir && !isFile {
			continue
		}

		if flag&C.kFSEventStreamEventFlagItemRenamed == C.kFSEventStreamEventFlagItemRenamed {
			if oldname == "" {
				oldname = abs
				oldisdir = isDir
				if i+1 < count {
					continue
				}
			} else {
				rename(oldname, abs)
				oldname = ""
				continue
			}
		}

		if oldname != "" {
			rel, _ := filepath.Rel(obs.root, oldname)
			if f, ok := obs.watched[rel]; ok {
				remove(f)
			} else {
				if oldisdir {
					watchDir(rel)
				} else {
					if f, err := add(oldname, false); err == nil {
						notify(fileCreate, f, "")
						notify(fileUpdate, f, "")
					}
				}
			}
			oldname = ""
		}

		if flag&(C.kFSEventStreamEventFlagItemCreated) != 0 {
			if isDir {
				watchDir(rel)
			} else {
				if f, err := add(abs, false); err == nil {
					notify(fileCreate, f, "")
				}
			}
		}

		if flag&C.kFSEventStreamEventFlagItemModified != 0 {
			if isFile {
				if f, ok := obs.watched[rel]; ok {
					notify(fileUpdate, f, "")
				}
			}
		}

		if flag&C.kFSEventStreamEventFlagItemRemoved != 0 {
			if f, ok := obs.watched[rel]; ok {
				remove(f)
			}
		}

		if flag&C.kFSEventStreamEventFlagMustScanSubDirs != 0 {
			core.LogInfo(errors.New("events coalesced, requiring rescan of subdirectories"))
			obs.watched = map[string]file{}
			watchDir(".")
		}
	}
}

// rename handles the rename of a file or directory.
func rename(oldname, newname string) {
	rel, _ := filepath.Rel(obs.root, oldname)
	reln, _ := filepath.Rel(obs.root, newname)
	f, ok := obs.watched[rel]
	if !ok {
		return
	}
	f.name = newname
	delete(obs.watched, rel)
	obs.watched[reln] = f
	if f.isDir {
		for rel, f := range obs.watched {
			n, err := filepath.Rel(oldname, f.name)
			if err == nil && strings.SplitN(n, string(filepath.Separator), 2)[0] != ".." {
				reln := filepath.Join(reln, n)
				delete(obs.watched, rel)
				f.name = filepath.Join(obs.root, reln)
				obs.watched[reln] = f
				if !f.isDir {
					notify(fileRename, f, rel)
				}
			}
		}
	} else {
		notify(fileRename, f, oldname)
	}
}

// addDir adds host specific handling for a directory.
func addDir(abs string) error {
	return nil
}

// removeDir removes host specific handling for a directory.
func removeDir(abs string) {
}
