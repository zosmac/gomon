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
	"path/filepath"
	"strconv"
	"unsafe"

	"github.com/zosmac/gocore"
)

// handle defines host specific observer properties.
type handle struct {
	stream C.FSEventStreamRef
}

// open obtains a directory handle for observer.
func open(_ string) (*handle, error) {
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
		return gocore.Error("FSEventStreamStart", errors.New("failed"), map[string]string{
			"directory": obs.root,
		})
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
	gocore.Error("FSEventStream monitoring", nil, map[string]string{
		"directory": C.GoString(&buf[0]),
	}).Info()

	return nil
}

// callback handles events sent on stream.
//
//export callback
func callback(stream C.ConstFSEventStreamRef, _ unsafe.Pointer, count C.size_t, paths **C.char, flags *C.FSEventStreamEventFlags, ids *C.FSEventStreamEventId) {
	pths := unsafe.Slice(paths, count)
	flgs := unsafe.Slice(flags, count)
	idss := unsafe.Slice(ids, count)

	oldf := file{}
	for i, path := range pths {
		abs := C.GoString(path)
		rel, err := filepath.Rel(obs.root, abs)
		if err != nil {
			continue
		}

		flag := flgs[i]
		id := strconv.Itoa(int(idss[i]))

		// do we care about symlink changes?
		isDir := flag&C.kFSEventStreamEventFlagItemIsDir == C.kFSEventStreamEventFlagItemIsDir
		isFile := flag&C.kFSEventStreamEventFlagItemIsFile == C.kFSEventStreamEventFlagItemIsFile
		if !isDir && !isFile {
			continue
		}

		if flag&C.kFSEventStreamEventFlagItemRenamed == C.kFSEventStreamEventFlagItemRenamed {
			if oldf.abs == "" {
				oldf = file{
					abs:   abs,
					isDir: isDir,
				}
				if i+1 < int(count) {
					continue // the next event should have the new name
				}
			} else {
				rename(id, abs, oldf.abs)
				oldf = file{}
				continue
			}
		}

		if oldf.abs != "" {
			rel, _ := filepath.Rel(obs.root, oldf.abs)
			if f, ok := obs.watched[rel]; ok {
				remove(f, id) // removed from where we are watching
			} else { // moved into where we are watching
				if oldf.isDir {
					watchDir(rel, id)
				} else {
					if f, err := add(oldf.abs, false); err == nil {
						notify(fileCreate, id, f.abs, "")
						notify(fileUpdate, id, f.abs, "")
					}
				}
			}
			oldf = file{}
		}

		if flag&(C.kFSEventStreamEventFlagItemCreated) != 0 {
			if isDir {
				watchDir(rel, id)
			} else {
				if f, err := add(abs, false); err == nil {
					notify(fileCreate, id, f.abs, "")
				}
			}
		}

		if flag&C.kFSEventStreamEventFlagItemModified != 0 {
			if isFile {
				if f, ok := obs.watched[rel]; ok {
					notify(fileUpdate, id, f.abs, "")
				}
			}
		}

		if flag&C.kFSEventStreamEventFlagItemRemoved != 0 {
			if f, ok := obs.watched[rel]; ok {
				remove(f, id)
			}
		}

		if flag&C.kFSEventStreamEventFlagMustScanSubDirs != 0 {
			gocore.Error("FSEvents coalesced", errors.New("subdirectories rescanned")).Info()
			obs.watched = map[string]file{}
			watchDir(".", id)
		}
	}
}

// rename handles the rename of a file or directory.
func rename(id, absn, abso string) {
	relo, _ := filepath.Rel(obs.root, abso)
	reln, _ := filepath.Rel(obs.root, absn)
	f, ok := obs.watched[relo]
	if !ok {
		return
	}

	f.abs = absn
	delete(obs.watched, relo)
	obs.watched[reln] = f

	if !f.isDir {
		notify(fileRename, id, absn, abso) // file renamed
		return
	}

	// check if directory moved, not simply renamed
	for relo, f := range obs.watched {
		if rel, err := gocore.Subdir(abso, f.abs); err == nil {
			abso := f.abs
			reln := filepath.Join(reln, rel)
			f.abs = filepath.Join(obs.root, reln)
			delete(obs.watched, relo)
			obs.watched[reln] = f
			if !f.isDir {
				notify(fileRename, id, absn, abso)
			}
		}
	}
}

// addDir adds host specific handling for a directory.
func addDir(_ string) error {
	return nil
}

// removeDir removes host specific handling for a directory.
func removeDir(_ string) {
}
