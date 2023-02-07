// Copyright © 2021-2023 The Gomon Project.

package process

import (
	"fmt"
	"strings"
)

// lsofCommand builds a host specific command line for lsof.
func lsofCommand() []string {
	return strings.Fields(fmt.Sprintf("lsof -l -n -P -X -r%dm====%%T====", 10))
}
