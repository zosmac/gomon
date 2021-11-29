// Copyright © 2021 The Gomon Project.

//go:build !windows

package core

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// signalChannel returns channel on which OS signals are delivered.
func signalChannel() <-chan os.Signal {
	signalChan := make(chan os.Signal, 1)                      // use buffered channel to ensure signal delivery
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM) // , syscall.SIGSEGV)
	signal.Ignore(syscall.SIGWINCH, syscall.SIGHUP, syscall.SIGTTIN, syscall.SIGTTOU)
	return signalChan
}

// setuid attempts to run the "gomon" command as owner (e.g. root) of executable.
func setuid() {
	uid := os.Getuid()
	euid := os.Geteuid()
	if uid != euid {
		if err := syscall.Setuid(euid); err == nil {
			uid = euid
		}
		LogInfo(fmt.Errorf("running as %s(%d)", Username(uid), uid))
	}
}
