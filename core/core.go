// Copyright © 2021 The Gomon Project.

package core

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"unsafe"
)

const (
	// TimeFormat used for formatting timestamps.
	TimeFormat = "2006-01-02T15:04:05.000Z07:00"
)

var (
	// HostEndian enables byte order conversion between local and network integers.
	HostEndian = func() binary.ByteOrder {
		n := uint16(1)
		a := (*[2]byte)(unsafe.Pointer(&n))[:]
		b := []byte{0, 1}
		if bytes.Equal(a, b) {
			return binary.BigEndian
		}
		return binary.LittleEndian
	}()
)

// ChDir is a convenience function for changing the current directory and reporting its canonical path.
// If changing the directory fails, ChDir returns the error and canonical path of the current directory.
func ChDir(dir string) (string, error) {
	var err error
	if dir, err = filepath.Abs(dir); err == nil {
		if dir, err = filepath.EvalSymlinks(dir); err == nil {
			if err = os.Chdir(dir); err == nil {
				return dir, nil
			}
		}
	}
	dir, _ = os.Getwd()
	dir, _ = filepath.EvalSymlinks(dir)
	return dir, err
}

// Wait waits for a started command and reports its completion status.
func Wait(cmd *exec.Cmd) {
	err := cmd.Wait()
	state := cmd.ProcessState
	LogInfo(fmt.Errorf(
		"Wait() command=%q pid=%d err=%v rc=%d\nsystime=%v, usrtime=%v, sys=%#v usage=%#v",
		cmd.String(),
		cmd.Process.Pid,
		err,
		state.ExitCode(),
		state.SystemTime(),
		state.UserTime(),
		state.Sys(),
		state.SysUsage(),
	))
}
