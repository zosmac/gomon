// Copyright © 2021 The Gomon Project.

package process

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// hostCommand builds a host specific command line for lsof.
func hostCommand() *exec.Cmd {
	cmdline := strings.Fields(fmt.Sprintf("lsof -l -n -P -d ^cwd,^rtd,^txt -r%dm====%%T====", 10))
	cmd := exec.Command(cmdline[0], cmdline[1:]...)

	dirname := filepath.Join("/proc", "self", "fd")
	if dir, err := os.Open(dirname); err == nil {
		fds, err := dir.Readdirnames(0)
		dir.Close()
		if err == nil {
			maxFd := -1
			for _, fd := range fds {
				if n, err := strconv.Atoi(fd); err == nil && n > maxFd {
					maxFd = n
				}
			}
			// ensure that no open descriptors propagate to child
			if maxFd >= 3 {
				cmd.ExtraFiles = make([]*os.File, maxFd-3) // close gomon files in child
			}
		}
	}

	return cmd
}
