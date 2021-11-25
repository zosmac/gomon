// Copyright © 2021 The Gomon Project.

package process

/*
#include <libproc.h>
*/
import "C"

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

var (
	fdinfos = func() []C.struct_proc_fdinfo {
		var rlimit syscall.Rlimit
		syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rlimit)
		return make([]C.struct_proc_fdinfo, rlimit.Cur)
	}()
)

// getInodes is called from Measure(), but is only relevant for Linux, so for Darwin is a noop.
func getInodes() {}

// hostCommand builds a host specific command line for lsof.
func hostCommand() *exec.Cmd {
	cmdline := strings.Fields(fmt.Sprintf("lsof -X -r%dm====%%T====", 10))
	cmd := exec.Command(cmdline[0], cmdline[1:]...)

	// ensure that no open descriptors propagate to child
	if n := C.proc_pidinfo(
		C.int(os.Getpid()),
		C.PROC_PIDLISTFDS,
		0,
		nil,
		0,
	); n >= 3*C.PROC_PIDLISTFD_SIZE {
		cmd.ExtraFiles = make([]*os.File, (n/C.PROC_PIDLISTFD_SIZE)-3) // close gomon files in child
	}

	return cmd
}
