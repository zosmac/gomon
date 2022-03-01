// Copyright © 2021 The Gomon Project.

package filesystem

/*
#include <sys/mount.h>
*/
import "C"

import (
	"syscall"
	"unsafe"

	"github.com/zosmac/gomon/core"
	"github.com/zosmac/gomon/message"
)

// filesystems returns a list of filesystems.
// See man pages for getfsstat and statfs for Darwin.
// func filesystems() ([]filesystem, error) {
func filesystems() ([]message.Request, error) {
	n, err := syscall.Getfsstat(nil, C.MNT_NOWAIT)
	if err != nil {
		return nil, core.Error("getfsstat", err)
	}
	fss := make([]syscall.Statfs_t, n)
	if _, err = syscall.Getfsstat(fss, C.MNT_NOWAIT); err != nil {
		return nil, core.Error("getfsstat", err)
	}

	var qs []message.Request
	for _, fs := range fss {
		fs := fs
		if fs.Flags&C.MNT_LOCAL == C.MNT_LOCAL {
			qs = append(qs,
				func() []message.Content {
					path := C.GoString((*C.char)(unsafe.Pointer(&fs.Mntonname[0])))
					return []message.Content{
						&measurement{
							Header: message.Measurement(sourceFilesystem),
							Id: Id{
								Mount: C.GoString((*C.char)(unsafe.Pointer(&fs.Mntfromname[0]))),
								Path:  path,
							},
							Properties: Properties{
								Type: C.GoString((*C.char)(unsafe.Pointer(&fs.Fstypename[0]))),
							},
							Metrics: metrics(path),
						},
					}
				},
			)
		}
	}

	return qs, nil
}
