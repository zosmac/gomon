// Copyright © 2021-2023 The Gomon Project.

package file

/*
#cgo CFLAGS: -x objective-c -std=gnu11 -fobjc-arc -D__unix__
#cgo LDFLAGS: -framework CoreServices
#import <CoreServices/CoreServices.h>
#include <dispatch/dispatch.h>

// Cannot resolve C.dispatch_queue_t type in Go code.
// Therefore, get queue and dispatch FS stream here.
static bool
QueueStream(FSEventStreamRef stream) {
	dispatch_queue_t queue;
	queue = dispatch_get_global_queue(QOS_CLASS_DEFAULT, 0);
	FSEventStreamSetDispatchQueue(stream, queue);
	return FSEventStreamStart(stream);
}

// callback handles events sent on FSEventStream
extern void callback(ConstFSEventStreamRef, void *, size_t, char **, FSEventStreamEventFlags *, FSEventStreamEventId *);
*/
import "C"

import (
	"errors"
	"fmt"
	"path/filepath"
	"unsafe"

	"github.com/zosmac/gocore"
)

// handle defines host specific observer properties.
type handle struct {
	stream C.FSEventStreamRef
}

// open obtains a directory handle for observer.
func open(directory string) (*handle, error) {
	cname := gocore.CreateCFString(flags.fileDirectory + "\x00")
	defer C.CFRelease(C.CFTypeRef(cname))
	context := C.malloc(C.sizeof_struct_FSEventStreamContext)
	defer C.free(context)
	C.memset(context, 0, C.sizeof_struct_FSEventStreamContext)

	return &handle{
		stream: C.FSEventStreamCreate(
			0,
			(*[0]byte)(C.callback),
			(*C.FSEventStreamContext)(context),
			C.CFArrayCreate(0, &cname, 1, nil),
			C.kFSEventStreamEventIdSinceNow,
			C.CFTimeInterval(1.0),
			C.kFSEventStreamCreateFlagFileEvents|C.kFSEventStreamCreateFlagWatchRoot, // |C.kFSEventStreamCreateFlagNoDefer,
		),
	}, nil
}

// close OS resources.
func (h *handle) close() {
	C.FSEventStreamStop(h.stream)
	C.FSEventStreamInvalidate(h.stream)
	C.FSEventStreamRelease(h.stream)
}

// observe events and notify observer's callbacks.
func observe() error {
	if ok := bool(C.QueueStream(obs.stream)); !ok {
		return gocore.Error("FSEventStreamStart failed", fmt.Errorf(obs.root))
	}

	paths := C.FSEventStreamCopyPathsBeingWatched(obs.stream)
	defer C.CFRelease(C.CFTypeRef(paths))
	var buf [1024]C.char
	C.CFStringGetCString(
		C.CFStringRef(C.CFArrayGetValueAtIndex(paths, 0)),
		&buf[0],
		C.CFIndex(len(buf)),
		C.kCFStringEncodingUTF8,
	)
	gocore.LogInfo("FSEventStream monitoring", errors.New(C.GoString(&buf[0])))

	return nil
}

// callback handles events sent on stream.
//
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
			gocore.LogInfo("FSEvents coalesced", errors.New("subdirectories rescanned"))
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
			if n, err := filepath.Rel(oldname, f.name); err == nil && (len(n) < 2 || n[:2] != "..") {
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
