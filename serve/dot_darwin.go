// Copyright © 2021-2023 The Gomon Project.

package serve

/*
#cgo CFLAGS: -I/usr/local/graphviz/include -I/Applications/GraphvizSwift.app/Contents/Frameworks/include
#cgo LDFLAGS: -L/usr/local/graphviz/lib -L/Applications/GraphvizSwift.app/Contents/Frameworks/lib -lgvc -lcgraph

#include <graphviz/gvc.h>
#include <stdlib.h>
*/
import "C"

import (
	"unsafe"

	"github.com/zosmac/gocore"
)

// dot calls the Graphviz dot command to render the process NodeGraph as SVG.
func dot(graphviz string) []byte {
	// // to debug the graph, write to a file
	// if cwd, err := os.Getwd(); err == nil {
	// 	if f, err := os.CreateTemp(cwd, "graphviz.*.gv"); err == nil {
	// 		os.Chmod(f.Name(), 0644)
	// 		f.WriteString(graphviz)
	// 		f.Close()
	// 	}
	// }

	graph := C.CString(graphviz)
	defer C.free(unsafe.Pointer(graph))

	gvc := C.gvContext()
	defer C.gvFreeContext(gvc)

	g := C.agmemread(graph)
	defer C.agclose(g)

	layout := C.CString("dot")
	defer C.free(unsafe.Pointer(layout))
	C.gvLayout(gvc, g, layout)
	defer C.gvFreeLayout(gvc, g)

	format := C.CString(output_format)
	defer C.free(unsafe.Pointer(format))

	var data *C.char
	var length C.size_t
	if rc, err := C.gvRenderData(gvc, g, format, &data, &length); rc != 0 {
		gocore.Error("dot", err).Err()
		return nil
	}
	defer C.gvFreeRenderData(data)

	return C.GoBytes(unsafe.Pointer(data), C.int(length))
}
