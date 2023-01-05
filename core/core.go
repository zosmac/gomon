// Copyright © 2021 The Gomon Project.

package core

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"unsafe"
)

type (
	// ValidValue defines list of values that are valid for a type safe string.
	ValidValue[T ~string] map[T]int
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

// Define initializes a ValidValue type with its valid values.
func (vv ValidValue[T]) Define(values ...T) ValidValue[T] {
	vv = map[T]int{}
	for i, v := range values {
		vv[v] = i
	}
	return vv
}

// ValidValues returns an ordered list of valid values for the type.
func (vv ValidValue[T]) ValidValues() []string {
	ss := make([]string, len(vv))
	for v, i := range vv {
		ss[i] = string(v)
	}
	return ss
}

// IsValid returns whether a value is valid.
func (vv ValidValue[T]) IsValid(v T) bool {
	_, ok := vv[v]
	return ok
}

// Index returns the position of a value in the valid value list.
func (vv ValidValue[T]) Index(v T) int {
	return vv[v]
}

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

// Wait for a started command to complete and report its exit status.
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

// IsTerminal reports if a file handle is connected to the terminal.
func IsTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	mode := info.Mode()

	// see https://github.com/golang/go/issues/23123
	if runtime.GOOS == "windows" {
		return mode&os.ModeCharDevice == os.ModeCharDevice
	}

	return mode&(os.ModeDevice|os.ModeCharDevice) == (os.ModeDevice | os.ModeCharDevice)
}
