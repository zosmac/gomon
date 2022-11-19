// Copyright © 2021 The Gomon Project.

package core

/*
 */
import "C"

// darkmode is called from viewDidChangeEffectiveAppearance to report changes to the system appearance.
// It must be defined separately from the declaration in core_darwin.go to prevent duplicate symbol link error.
// From the CGO documentation (https://golang.google.cn/cmd/cgo#hdr-C_references_to_Go):
//
//	Using //export in a file places a restriction on the preamble: since it is copied into two different
//	C output files, it must not contain any definitions, only declarations. If a file contains both definitions
//	and declarations, then the two output files will produce duplicate symbols and the linker will fail. To avoid
//	this, definitions must be placed in preambles in other files, or in C source files.
//
//export darkmode
func darkmode(dark bool) {
	DarkAppearance = dark
}
