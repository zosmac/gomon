// Copyright © 2021-2023 The Gomon Project.

//go:build !darwin

package serve

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

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

	cmd := exec.Command("dot", "-v", "-T"+output_format)
	cmd.Stdin = bytes.NewBufferString(graphviz)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		gocore.Error("dot", err, map[string]string{
			"stderr": stderr.String(),
		}).Err()
		sc := bufio.NewScanner(strings.NewReader(graphviz))
		for i := 1; sc.Scan(); i++ {
			fmt.Fprintf(os.Stderr, "%4.d %s\n", i, sc.Text())
		}
		return nil
	}
	return stdout.Bytes()
}
