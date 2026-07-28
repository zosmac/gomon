// Copyright © 2021-2023 The Gomon Project.

package message

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/zosmac/gocore"
)

// Protobuf defines the protocol buffer message structures for gRPC when the protobuf flag specified on the command line.
func Protobuf() {
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

	fmt.Print(`edition = "2024";
package main;
option go_package = "github.com/zosmac/gomon";
import "google/protobuf/duration.proto";
import "google/protobuf/timestamp.proto";

service Gomon {
  rpc GetMessages(MessageTypes) returns (stream Message) {}
}

message MessageTypes {
	int32 Types = 1;
}
`)

	fmt.Println(buildProto())
}

func buildProto() string {
	prevMessage := ""
	firstProperty := true
	firstMetric := true
	index := 1
	saveIndex := []int{}
	depth := 0
	indent := "  "
	var copyFields strings.Builder

	for _, f := range fields {
		if f.key != prevMessage {
			if !firstProperty || !firstMetric {
				for ; depth >= 0; depth-- {
					indent = indent[:len(indent)-2]
					fmt.Println(indent + "}") // finish previous message definition
				}
				copyFields.WriteString("  }\n}\n")
			}
			key := strings.Split(f.key, " |")
			event := "Measurement"
			if len(key) > 1 && key[1] != "measure" {
				event = "Observation"
			}
			fmt.Printf("\nmessage %s%s {\n", gocore.Capitalize(key[0]), event)
			fmt.Fprintf(&copyFields, "\nfunc Copy%s%s(src *%s.%[2]s) %[1]s%s {\n", gocore.Capitalize(key[0]), event, key[0])
			fmt.Fprintf(&copyFields, "  return %s%s{\n", gocore.Capitalize(key[0]), event)

			prevMessage = f.key
			firstProperty = true
			firstMetric = true
			index = 1
			saveIndex = []int{}
			depth = 0
			indent = "  "
		}

		if f.Property {
			if firstProperty {
				firstProperty = false
			}
		} else {
			if firstMetric {
				firstMetric = false
			}
		}

		var typ, name string
		nestedMessage := ""
		switch f.Value.Type().Name() {
		case "int":
			typ = "int64"
		case "float64":
			typ = "double"
		case "Time":
			typ = "google.protobuf.Timestamp"
		case "Duration":
			typ = "google.protobuf.Duration"
		case "Pid":
			typ = "int64"
		default:
			typ = f.Value.Kind().String()
			if typ == "struct" {
				typ = f.Value.Type().Name()
				nestedMessage = "message " + typ + " {\n"
			}
		}

		if strings.HasSuffix(f.Name, "[n]") {
			name = f.Name[:len(f.Name)-3]
			typ = "repeated " + typ
		} else if n := strings.Index(f.Name, "[key]"); n >= 0 {
			gocore.Error("Protobuf", fmt.Errorf("map types are not yet supported for protobuf")).Err()
			typ = "map<" + f.Value.Type().Key().Name() + ", " + f.Value.Type().Elem().Name() + ">"
			name = f.Name[:n]
		} else {
			name = f.Name
		}

		for i, c := range name {
			if c == ' ' {
				continue
			}
			for ; depth > i/2; depth-- {
				indent = indent[:len(indent)-2]
				fmt.Println(indent + "}") // finish previous message definition
				index = saveIndex[len(saveIndex)-1]
				saveIndex = saveIndex[:len(saveIndex)-1]
			}
			break
		}

		fmt.Printf("%s%s %s = %d;\n", indent, typ, strings.TrimSpace(name), index)
		index++
		if nestedMessage != "" {
			fmt.Printf("%s%s", indent, nestedMessage)
			indent += "  "
			depth += 1
			saveIndex = append(saveIndex, index)
			index = 1
			fmt.Fprintf(&copyFields, "    %s: src.%[1]s,\n", strings.TrimSpace(name))
		} else if depth == 0 {
			fmt.Fprintf(&copyFields, "    %s: src.%[1]s,\n", strings.TrimSpace(name))
		}
	}

	if prevMessage != "" {
		if !firstProperty || !firstMetric {
			for ; depth >= 0; depth-- {
				indent = indent[:len(indent)-2]
				fmt.Println(indent + "}") // finish previous message definition
			}
			copyFields.WriteString("  }\n}\n")
		}
	}

	return copyFields.String()
}
