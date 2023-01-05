// Copyright © 2021 The Gomon Project.

package core

/*
#cgo CFLAGS: -x objective-c -std=gnu11 -fobjc-arc
#cgo LDFLAGS: -framework CoreFoundation -framework AppKit -framework Foundation -framework IOKit
#import <CoreFoundation/CoreFoundation.h>
#import <Foundation/Foundation.h>
#import <AppKit/AppKit.h>
#include <libproc.h>
#include <sys/sysctl.h>

void *CopyCFString(char *s) { // required to avoid go vet "possible misuse of unsafe.Pointer"
	return (void*)CFStringCreateWithCString(nil, s, kCFStringEncodingUTF8);
}

// The following code invoked by C.Run() is to capture dynamically the system appearance.
// Simply querying the NSApplication effective appearance is insufficient if no view defined.

extern void darkmode(BOOL);

@interface MyView : NSView
@end

@implementation MyView
- (id)initWithFrame:(CGRect)frame
{
    self = [super initWithFrame:frame];
    return self;
}
- (void)viewDidChangeEffectiveAppearance {
	NSString *name = [[self effectiveAppearance] name];
	NSLog(@"Changed Appearance is %@", name);
	name = [[self effectiveAppearance] bestMatchFromAppearancesWithNames: @[NSAppearanceNameAqua, NSAppearanceNameDarkAqua]];
	darkmode((name == NSAppearanceNameDarkAqua) ? TRUE : FALSE);
}
@end

@interface MyAppDelegate : NSObject<NSApplicationDelegate>
@end

@implementation MyAppDelegate
- (void)applicationDidFinishLaunching:(NSNotification*)note {
	NSLog(@"Finish launch notification is %@", [note description]);
}
- (NSApplicationTerminateReply)applicationShouldTerminate:(NSApplication *)app {
	NSLog(@"Application should terminate %@", [app description]);
	return NSTerminateNow;
}
@end

static void
Run() {
	[NSApplication sharedApplication]; // initialize the application
	[NSApp setDelegate: [[MyAppDelegate alloc] init]];

	NSRect rect = NSMakeRect(100.0, 100.0, 100.0, 100.0);
	NSWindow *window = [[NSWindow alloc]
		initWithContentRect:rect
                  styleMask:NSWindowStyleMaskTitled|NSWindowStyleMaskClosable
                    backing:NSBackingStoreBuffered
                      defer:NO
	];

	rect = [[window contentView] frame];
	MyView *view = [[MyView alloc] initWithFrame: rect];
	[window setContentView: view];
	[NSApp hide:nil];
	[NSApp run];
}

static void
terminate() {
	[NSApp terminate: nil];
}
*/
import "C"

import (
	"context"
	"fmt"
	"os"
	"syscall"
	"time"
	"unsafe"
)

// CGO interprets C. types in different packages as being different types. The following core package aliases for
// CoreFoundation (C.CF...) types externalize these locally defined types as core package types. Casting C.CF...Ref
// arguments to core.CF...Ref enables callers from other packages by using the core package type name.
type (
	// CFStringRef creates core package alias for type
	CFStringRef = C.CFStringRef
	// CFNumberRef creates core package alias for type
	CFNumberRef = C.CFNumberRef
	// CFBooleanRef creates core package alias for type
	CFBooleanRef = C.CFBooleanRef
	// CFArrayRef creates core package alias for type
	CFArrayRef = C.CFArrayRef
	// CFDictionaryRef creates core package alias for type
	CFDictionaryRef = C.CFDictionaryRef
)

var (
	// Boottime retrieves the system boot time.
	Boottime = func() time.Time {
		var timespec C.struct_timespec
		size := C.size_t(C.sizeof_struct_timespec)
		if rv, err := C.sysctl(
			(*C.int)(unsafe.Pointer(&[2]C.int{C.CTL_KERN, C.KERN_BOOTTIME})),
			2,
			unsafe.Pointer(&timespec),
			&size,
			unsafe.Pointer(nil),
			0,
		); rv != 0 {
			LogError(Error("sysctl kern.boottime", err))
			return time.Time{}
		}

		return time.Unix(int64(timespec.tv_sec), int64(timespec.tv_nsec))
	}()
)

// FdPath gets the path for an open file descriptor.
func FdPath(fd int) (string, error) {
	pid := C.int(os.Getpid())
	var fdi C.struct_vnode_fdinfowithpath
	if n, err := C.proc_pidfdinfo(
		pid,
		C.int(fd),
		C.PROC_PIDFDVNODEPATHINFO,
		unsafe.Pointer(&fdi),
		C.PROC_PIDFDVNODEPATHINFO_SIZE,
	); n <= 0 {
		return "", Error("proc_pidfdinfo PROC_PIDFDVNODEPATHINFO failed", err)
	}
	return C.GoString(&fdi.pvip.vip_path[0]), nil
}

// MountMap builds a map of mount points to file systems.
func MountMap() (map[string]string, error) {
	n, err := syscall.Getfsstat(nil, C.MNT_NOWAIT)
	if err != nil {
		return nil, Error("getfsstat", err)
	}
	list := make([]syscall.Statfs_t, n)
	if _, err = syscall.Getfsstat(list, C.MNT_NOWAIT); err != nil {
		return nil, Error("getfsstat", err)
	}

	m := map[string]string{"/": ""} // have "/" at a minimum
	for _, f := range list[0:n] {
		if f.Blocks == 0 {
			continue
		}
		m[C.GoString((*C.char)(unsafe.Pointer(&f.Mntonname[0])))] =
			C.GoString((*C.char)(unsafe.Pointer(&f.Mntfromname[0])))
	}
	return m, nil
}

// CreateCFString copies a Go string as a Core Foundation CFString. Requires CFRelease be called when done.
func CreateCFString(s string) unsafe.Pointer {
	cs := C.CString(s)
	defer C.free(unsafe.Pointer(cs))
	return unsafe.Pointer(C.CopyCFString(cs))
}

// GetCFString gets a Go string from a CFString
func GetCFString(p CFStringRef) string {
	if p == 0 {
		return ""
	}

	if s := C.CFStringGetCStringPtr(p, C.kCFStringEncodingUTF8); s != nil {
		return C.GoString(s)
	}

	var buf [1024]C.char
	C.CFStringGetCString(
		p,
		&buf[0],
		C.CFIndex(len(buf)),
		C.kCFStringEncodingUTF8,
	)
	return C.GoString(&buf[0])
}

// GetCFNumber gets a Go numeric type from a CFNumber
func GetCFNumber(n CFNumberRef) interface{} {
	var i int64
	var f float64
	t := C.CFNumberType(C.kCFNumberSInt64Type)
	p := unsafe.Pointer(&i)
	v := interface{}(&i)
	if C.CFNumberIsFloatType(n) == C.true {
		t = C.kCFNumberFloat64Type
		p = unsafe.Pointer(&f)
		v = interface{}(&f)
	}
	C.CFNumberGetValue(n, t, p)
	if _, ok := v.(*int64); ok {
		return i
	}
	return f
}

// GetCFBoolean gets a Go bool from a CFBoolean
func GetCFBoolean(b CFBooleanRef) bool {
	return C.CFBooleanGetValue(b) != 0
}

// GetCFArray gets a Go slice from a CFArray
func GetCFArray(a CFArrayRef) []interface{} {
	c := C.CFArrayGetCount(a)
	s := make([]interface{}, c)
	vs := make([]unsafe.Pointer, c)
	C.CFArrayGetValues(a, C.CFRange{length: c, location: 0}, &vs[0])

	for i := 0; i < int(c); i++ {
		s[i] = GetCFValue(vs[i])
	}

	return s
}

// GetCFDictionary gets a Go map from a CFDictionary
func GetCFDictionary(d CFDictionaryRef) map[string]interface{} {
	if d == 0 {
		return nil
	}
	c := int(C.CFDictionaryGetCount(d))
	m := make(map[string]interface{}, c)
	ks := make([]unsafe.Pointer, c)
	vs := make([]unsafe.Pointer, c)
	C.CFDictionaryGetKeysAndValues(d, &ks[0], &vs[0])

	for i := 0; i < c; i++ {
		if C.CFGetTypeID(C.CFTypeRef(ks[i])) != C.CFStringGetTypeID() {
			continue
		}
		k := GetCFString(CFStringRef(ks[i]))
		m[k] = GetCFValue(vs[i])
	}

	return m
}

func GetCFValue(v unsafe.Pointer) interface{} {
	switch id := C.CFGetTypeID(C.CFTypeRef(v)); id {
	case C.CFStringGetTypeID():
		return GetCFString(CFStringRef(v))
	case C.CFNumberGetTypeID():
		return GetCFNumber(CFNumberRef(v))
	case C.CFBooleanGetTypeID():
		return GetCFBoolean(CFBooleanRef(v))
	case C.CFDictionaryGetTypeID():
		return GetCFDictionary(CFDictionaryRef(v))
	case C.CFArrayGetTypeID():
		return GetCFArray(CFArrayRef(v))
	default:
		d := C.CFCopyDescription(C.CFTypeRef(v))
		t := GetCFString(d)
		C.CFRelease(C.CFTypeRef(d))
		return fmt.Sprintf("Unrecognized Type is %d: %s\n", id, t)
	}
}

// osEnvironment starts the native application environment run loop.
// Note that a native application environment runs on the main thread.
// Therefore, launch the gomon command Main() in a go routine.
func osEnvironment(ctx context.Context) {
	go func() {
		<-ctx.Done()
		C.terminate()
	}()
	C.Run()
}
