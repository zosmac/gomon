// Copyright © 2021 The Gomon Project.

package process

/*
#include <libproc.h>
*/
import "C"

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// hostCommand builds a host specific command line for lsof.
func hostCommand(ctx context.Context) *exec.Cmd {
	cmdline := strings.Fields(fmt.Sprintf("lsof -l -n -P -X -r%dm====%%T====", 10))
	cmd := exec.CommandContext(ctx, cmdline[0], cmdline[1:]...)

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
