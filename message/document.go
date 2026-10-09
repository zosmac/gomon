// Copyright © 2021-2023 The Gomon Project.

package message

import (
	"fmt"
	"slices"
	"strings"
)

var (
	// max holds the maximum length for formatting of each field name, unit, and type.
	max = struct {
		Name int
		Unit int
		Type int
	}{len("- properties "), len(" units "), len(" type ")}
)

// Document the messages when the document flag specified on the command line.
func Document() {
	keys := make([]string, 0, len(messages))
	for key := range messages {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	headers := []string{
		fmt.Sprintf("+-%s%s-+\n",
			"- properties ", strings.Repeat("-", max.Name-len("- properties ")),
		),
		fmt.Sprintf("+-%s%s-+-%s%s-+-%s%s-+\n",
			"- metrics ", strings.Repeat("-", max.Name-len("- metrics ")),
			" type ", strings.Repeat("-", max.Type-len(" type ")),
			" units ", strings.Repeat("-", max.Unit-len(" units ")),
		),
	}
	footers := []string{
		fmt.Sprintf("+-%s-+\n",
			strings.Repeat("-", max.Name),
		),
		fmt.Sprintf("+-%s-+-%s-+-%s-+\n",
			strings.Repeat("-", max.Name),
			strings.Repeat("-", max.Type),
			strings.Repeat("-", max.Unit),
		),
	}

	for _, key := range keys {
		k := strings.Split(key, " |")
		fmt.Printf(
			"Source: %s\nEvents: %v\n",
			k[0],
			strings.Split(k[1], "|"),
		)
		firstProperty := true
		firstMetric := true

		for _, f := range messages[key] {
			if f.Property {
				if firstProperty {
					fmt.Print(headers[0])
					firstProperty = false
				}
				fmt.Printf("| %-*s |\n", max.Name, f.Name)
			} else {
				if firstMetric {
					fmt.Print(headers[1])
					firstMetric = false
				}
				fmt.Printf("| %-*s | %-*s | %-*s |\n",
					max.Name, f.Name,
					max.Type, f.Type,
					max.Unit, f.Unit,
				)
			}
		}
		if !firstProperty || !firstMetric {
			if firstMetric {
				fmt.Println(footers[0]) // finish previous table
			} else {
				fmt.Println(footers[1]) // finish previous table
			}
		}
	}
}
