// Copyright © 2021-2023 The Gomon Project.

package message

import (
	"cmp"
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/zosmac/gocore"
)

var (
	// Messages contains a map of all message definitions.
	Messages = map[string][]field{}

	// fields contains a definition for each message's fields.
	fields []field

	// max holds the maximum length for formatting of each field name, unit, and type.
	max = struct {
		Name int
		Unit int
		Type int
	}{len("- properties "), len(" units "), len(" type ")}
)

// Define a Message's Content.
func Define(m Content) {
	fs := gocore.Format("", "", reflect.ValueOf(m),
		func(name, tag string, val reflect.Value) any {
			return documentField(m, name, tag)
		},
	)
	src := filepath.Base(reflect.ValueOf(m).Elem().Type().PkgPath())
	k := src + " |" + strings.Join(m.Events(), "|")
	Messages[k] = make([]field, len(fs))
	for i, f := range fs {
		Messages[k][i] = f.(field)
		fields = append(fields, f.(field))
	}
}

// field records the attributes of a field for documenting.
type field struct {
	key      string
	Name     string
	Property bool   // true if field is a property
	Type     string // metric type
	Unit     string // metric unit
}

// Document the messages when the document flag specified on the command line.
func Document() {
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

// documentField interprets a gomon tag for the Document Formatter.
func documentField(m Content, name, tag string) field {
	if max.Name < len(name) {
		max.Name = len(name)
	}

	s := strings.Split(tag, ",")
	t := "counter"
	u := ""
	if len(s) > 0 {
		t = s[0]
	}
	if len(s) > 1 {
		u = s[1]
	}

	key := filepath.Base(reflect.ValueOf(m).Elem().Type().PkgPath()) + " |" + strings.Join(m.Events(), "|")

	if t == "property" {
		return field{
			key:      key,
			Name:     name,
			Property: true,
		}
	}

	if max.Type < len(t) {
		max.Type = len(t)
	}
	if max.Unit < len(u) {
		max.Unit = len(u)
	}

	return field{
		key:  key,
		Name: name,
		Type: t,
		Unit: u,
	}
}
