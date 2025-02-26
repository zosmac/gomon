// Copyright © 2021-2023 The Gomon Project.

package process

import (
	"fmt"
	"strings"
	"time"

	"github.com/zosmac/gocore"
)

// lsofCommand builds a host specific command line for lsof.
func lsofCommand() []string {
	sample, _ := time.ParseDuration(gocore.Flags.Lookup("sample").Value.String())
	return strings.Fields(fmt.Sprintf("lsof +c0 -l -n -P -X -r%dm====%%T====", sample/time.Second))
}
