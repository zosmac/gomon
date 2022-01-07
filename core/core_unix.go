// Copyright © 2021 The Gomon Project.

//go:build !windows
// +build !windows

package core

import (
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
