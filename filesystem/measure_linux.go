// Copyright © 2021-2023 The Gomon Project.

package filesystem

import (
	"bufio"
	"os"
	"strings"

	"github.com/zosmac/gomon/message"
)

var (
	// deviceTypes used on this system.
	deviceTypes = map[string]struct{}{}
)

// init builds a list of the filesystem device types.
func init() {
	f, err := os.Open("/proc/filesystems")
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		f := strings.Split(sc.Text(), "\t")
		deviceTypes[f[1]] = struct{}{}
	}
}

// filesystems returns a list of filesystems.
func filesystems() ([]message.Request, error) {
	m, err := os.Open("/etc/mtab")
	if err != nil {
		return nil, err
	}
	defer m.Close()

	var qs []message.Request
	sc := bufio.NewScanner(m)
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if _, ok := deviceTypes[f[2]]; !ok {
			continue
		}
		qs = append(qs,
			func() []message.Content {
				return []message.Content{
					&measurement{
						Header: message.Measurement(),
						Id: Id{
							Mount: f[0],
							Path:  f[1],
						},
						Properties: Properties{
							Type:    f[2],
							Options: f[3],
						},
						Metrics: metrics(f[1]),
					},
				}
			},
		)
	}

	return qs, nil
}
