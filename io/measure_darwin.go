// Copyright © 2021 The Gomon Project.

package io

/*
#cgo CFLAGS: -x objective-c -std=gnu11 -fobjc-arc
#cgo LDFLAGS: -framework CoreFoundation -framework IOKit
#import <CoreFoundation/CoreFoundation.h>
#import <IOKit/IOKitLib.h>
#import <IOKit/storage/IOBlockStorageDriver.h>
#import <IOKit/storage/IOMedia.h>
#import <IOKit/IOBSD.h>
*/
import "C"

import (
	"runtime"
	"strconv"
	"time"
	"unsafe"

	"github.com/zosmac/gomon/core"
	"github.com/zosmac/gomon/message"
)

// Measure captures system's I/O metrics.
func Measure() (ms []message.Content) {
	runtime.LockOSThread() // tie this goroutine to an OS thread
	defer runtime.UnlockOSThread()

	plane := C.CString(C.kIOServicePlane)
	defer C.free(unsafe.Pointer(plane))
	keyStats := core.CreateCFString(C.kIOBlockStorageDriverStatisticsKey)
	defer C.CFRelease(C.CFTypeRef(keyStats))

	var drives C.io_iterator_t
	class := C.CString(C.kIOBlockStorageDriverClass)
	C.IOServiceGetMatchingServices(
		C.kIOMasterPortDefault,
		C.CFDictionaryRef(C.IOServiceMatching(class)),
		&drives,
	)
	C.free(unsafe.Pointer(class))

	for {
		drive := C.IOIteratorNext(drives)
		if drive == 0 {
			break
		}

		var children C.io_iterator_t
		if C.IORegistryEntryGetChildIterator(drive, plane, &children) != 0 {
			continue
		}

		var dictionary C.CFDictionaryRef
		if C.IORegistryEntryCreateCFProperties(
			drive,
			(*C.CFMutableDictionaryRef)(&dictionary),
			C.kCFAllocatorDefault,
			C.kNilOptions,
		) != 0 {
			continue
		}

		for {
			child := C.IOIteratorNext(children)
			if child == 0 {
				break
			}

			var properties core.CFDictionaryRef
			if C.IORegistryEntryCreateCFProperties(
				child,
				(*C.CFMutableDictionaryRef)(&properties),
				C.kCFAllocatorDefault,
				C.kNilOptions,
			) != 0 {
				continue
			}
			cp := core.GetCFDictionary(properties)
			C.CFRelease(C.CFTypeRef(properties))

			name, ok := cp[C.kIOBSDNameKey]
			if !ok {
				continue
			}

			major := cp[C.kIOBSDMajorKey]
			minor := cp[C.kIOBSDMinorKey]
			size := cp[C.kIOMediaSizeKey]
			blockSize := cp[C.kIOMediaPreferredBlockSizeKey]

			stats := core.GetCFDictionary(core.CFDictionaryRef(C.CFDictionaryGetValue(dictionary, keyStats)))
			if stats == nil {
				continue
			}
			reads := stats[C.kIOBlockStorageDriverStatisticsReadsKey]
			bytesRead := stats[C.kIOBlockStorageDriverStatisticsBytesReadKey]
			totalReadTime := stats[C.kIOBlockStorageDriverStatisticsTotalReadTimeKey]
			writes := stats[C.kIOBlockStorageDriverStatisticsWritesKey]
			bytesWritten := stats[C.kIOBlockStorageDriverStatisticsBytesWrittenKey]
			totalWriteTime := stats[C.kIOBlockStorageDriverStatisticsTotalWriteTimeKey]

			ms = append(ms, &measurement{
				Header: message.Measurement(),
				Id: Id{
					Device: name.(string),
				},
				Properties: Properties{
					Major:     strconv.FormatInt(major.(int64), 10),
					Minor:     strconv.FormatInt(minor.(int64), 10),
					TotalSize: int(size.(int64)),
					BlockSize: int(blockSize.(int64)),
				},
				Metrics: Metrics{
					ReadOperations:  int(reads.(int64)),
					Read:            int(bytesRead.(int64)),
					ReadTime:        time.Duration(totalReadTime.(int64)),
					WriteOperations: int(writes.(int64)),
					Write:           int(bytesWritten.(int64)),
					WriteTime:       time.Duration(totalWriteTime.(int64)),
				},
			})
		}

		C.CFRelease(C.CFTypeRef(dictionary))
	}

	return
}
