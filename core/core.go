// Copyright © 2021 The Gomon Project.

package core

import (
	"bytes"
	"encoding/binary"
	"os"
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

// Htons converts host short to network format.
func Htons(h uint16) uint16 {
	return binary.BigEndian.Uint16((*(*[2]uint8)(unsafe.Pointer(&h)))[:])
}

// Htonl converts host long to network format.
func Htonl(h uint32) uint32 {
	return binary.BigEndian.Uint32((*(*[4]uint8)(unsafe.Pointer(&h)))[:])
}

// Ntohs converts network short to host format.
func Ntohs(n uint16) uint16 {
	return binary.BigEndian.Uint16((*(*[2]uint8)(unsafe.Pointer(&n)))[:])
}

// Ntohl converts network long to host format.
func Ntohl(n uint32) uint32 {
	return binary.BigEndian.Uint32((*(*[4]uint8)(unsafe.Pointer(&n)))[:])
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
