// Copyright © 2021 The Gomon Project.

//go:build !windows

package core

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

var (
	// euid gets the executable's owner id.
	euid = os.Geteuid()
)

// signalChannel returns channel on which OS signals are delivered.
func signalChannel() <-chan os.Signal {
	signalChan := make(chan os.Signal, 1)                      // use buffered channel to ensure signal delivery
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM) // , syscall.SIGSEGV)
	signal.Ignore(syscall.SIGWINCH, syscall.SIGHUP, syscall.SIGTTIN, syscall.SIGTTOU)
	return signalChan
}

// seteuid gomon-datasource to owner.
func Seteuid() {
	err := syscall.Seteuid(euid)
	LogInfo(fmt.Errorf("Seteuid results, uid: %d, euid: %d, err: %v",
		os.Getuid(),
		os.Geteuid(),
		err,
	))
}

// setuid gomon-datasource to grafana user.
func Setuid() {
	err := syscall.Seteuid(os.Getuid())
	LogInfo(fmt.Errorf("Setuid results, uid: %d, euid: %d, err: %v",
		os.Getuid(),
		os.Geteuid(),
		err,
	))
}
