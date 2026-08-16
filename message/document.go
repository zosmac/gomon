// Copyright © 2021-2023 The Gomon Project.

package message

import (
	"cmp"
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
	// TODO: this fields sort is both here and in protobuf.go. It should go refactored to somewhere in gocore.
	slices.SortStableFunc(fields, func(a, b field) int {
		if c := cmp.Compare(a.key, b.key); c != 0 {
			return c
		}
		if a.Property == b.Property {
			return 0
		} else if a.Property {
			return -1
		}
		return 1 // b is a property
	})

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

	prevMessage := ""
	firstProperty := true
	firstMetric := true

	for _, f := range fields {
		if f.key != prevMessage {
			if !firstProperty || !firstMetric {
				if firstMetric {
					fmt.Println(footers[0]) // finish previous table
				} else {
					fmt.Println(footers[1]) // finish previous table
				}
			}
			key := strings.Split(f.key, " |")
			fmt.Printf(
				"Source: %s\nEvents: %v\n",
				key[0],
				strings.Split(key[1], "|"),
			)
			prevMessage = f.key
			firstProperty = true
			firstMetric = true
		}
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

	if prevMessage != "" {
		if !firstProperty || !firstMetric {
			if firstMetric {
				fmt.Println(footers[0]) // finish previous table
			} else {
				fmt.Println(footers[1]) // finish previous table
			}
		}
	}
}
