// Copyright © 2021-2023 The Gomon Project.

package message

import (
	"cmp"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strings"

	"github.com/zosmac/gocore"
)

type (
	Node struct {
		message  string
		fields   []field
		parent   *Node
		children Nodes
	}

	Nodes map[string]*Node
)

// Protobuf defines the protocol buffer message structures for gRPC when the protobuf flag specified on the command line.
func Protobuf() {
	pbFile, err := os.Create("proto/gomon.proto")
	if err != nil {
		gocore.Error("Protobuf", err).Err()
		return
	}
	defer pbFile.Close()

	goFile, err := os.Create("proto/gomon.pbfn.go")
	if err != nil {
		gocore.Error("Protobuf", err).Err()
		return
	}
	defer goFile.Close()

	messages := new(strings.Builder)
	functions := new(strings.Builder)
	headers(messages, functions)
	messagesFunctions(messages, functions)
	footers(messages, functions)
	pbFile.WriteString(messages.String())
	goFile.WriteString(functions.String())
}

func headers(messages *strings.Builder, functions *strings.Builder) {
	// TODO: this fields sort is both here and in document.go. It should be refactored to somewhere in gocore.
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

	messages.WriteString(`edition = "2024";
package proto;
option go_package = ".;proto";
import "google/protobuf/duration.proto";
import "google/protobuf/timestamp.proto";

service Gomon {
  rpc GetMessages(GomonMessageTypes) returns (stream GomonMessage) {}
}

message GomonMessageTypes {
  int32 types = 1;
}

message GomonMessage {
  oneof gomon_message {
	FileObservation file_observation = 1;
    FilesystemMeasurement filesystem_measurement = 2;
    IoMeasurement io_measurement = 3;
	LogsObservation logs_observation = 4;
    NetworkMeasurement network_measurement = 5;
	ProcessMeasurement process_measurement = 6;
	ProcessObservation process_observation = 7;
	ServeMeasurement serve_measurement = 8;
	SystemMeasurement system_measurement = 9;
  }
} 
`)

	functions.WriteString(`package proto

import (
	"github.com/zosmac/gomon/file"
	"github.com/zosmac/gomon/filesystem"
	"github.com/zosmac/gomon/io"
	"github.com/zosmac/gomon/logs"
	"github.com/zosmac/gomon/network"
	"github.com/zosmac/gomon/process"
	"github.com/zosmac/gomon/serve"
	"github.com/zosmac/gomon/system"
	durationpb "google.golang.org/protobuf/types/known/durationpb"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
)

func toInt64(n int) *int64 {
	i := int64(n)
	return &i
}
`)
}

func footers(messages *strings.Builder, functions *strings.Builder) {
}

func messagesFunctions(messages *strings.Builder, functions *strings.Builder) {
	prevMessage := ""
	depth := 0
	node := &Node{}
	tree := Nodes{}

	for _, f := range fields {
		if f.key != prevMessage {
			if prevMessage != "" {
				functions.WriteString("  }\n  return b.Build()\n}\n")
			}
			key := strings.Split(f.key, " |")
			event := "Measurement"
			if len(key) > 1 && key[1] != "measure" {
				event = "Observation"
			}
			typ := gocore.Capitalize(key[0]) + event

			fmt.Fprintf(functions, "\nfunc Copy%s(src *%s.%s) *%[1]s {\n", typ, key[0], event)
			fmt.Fprintf(functions, "  b := %s_builder {\n", typ)

			prevMessage = f.key
			depth = 0
			tree[typ] = &Node{
				message:  typ,
				fields:   []field{},
				parent:   nil,
				children: Nodes{},
			}
			node = tree[typ]
		}

		nestedMessage := false
		if f.Value.Kind().String() == "struct" && f.Value.Type().Name() != "Time" {
			nestedMessage = true
		}

		var name string
		if strings.HasSuffix(f.Name, "[n]") {
			name = f.Name[:len(f.Name)-3]
			fmt.Fprintf(os.Stderr, "slice of type %s\n", f.Value.Type())
			f.Value = reflect.MakeSlice(reflect.SliceOf(f.Value.Type()), 0, 0)
		} else if n := strings.Index(f.Name, "[key]"); n >= 0 {
			gocore.Error("Protobuf", fmt.Errorf("map types support for protobuf unverified")).Err()
			name = f.Name[:n]
		} else {
			name = f.Name
		}

		for i, c := range name {
			if c == ' ' {
				continue
			}
			for ; depth > i/2; depth-- {
				node = node.parent
			}
			break
		}

		if depth == 0 {
			fmt.Fprintf(os.Stderr, "%s %s\n", strings.TrimSpace(name), f.Value.Kind().String())

			if f.Value.Type().Name() == "Time" {
				fmt.Fprintf(functions, "    %s: timestamppb.New(src.%[1]s),\n", strings.TrimSpace(name))
			} else if f.Value.Type().Name() == "Duration" {
				fmt.Fprintf(functions, "    %s: durationpb.New(src.%[1]s),\n", strings.TrimSpace(name))
			} else if f.Value.Kind().String() == "int" {
				fmt.Fprintf(functions, "    %s: toInt64(int(src.%[1]s)),\n", strings.TrimSpace(name))
			} else if f.Value.Kind().String() == "struct" {
				fmt.Fprintf(functions, "    %s: &src.%[1]s,\n", strings.TrimSpace(name))
			} else if f.Value.Kind().String() == "slice" {
				fmt.Fprintf(functions, "    %s: src.%[1]s,\n", strings.TrimSpace(name))
			} else if f.Value.Kind().String() == "string" && f.Value.Type().Name() != "string" {
				fmt.Fprintf(functions, "    %s: (*string)(&src.%[1]s),\n", strings.TrimSpace(name))
			} else {
				fmt.Fprintf(functions, "    %s: &src.%[1]s,\n", strings.TrimSpace(name))
			}
		}

		node.fields = append(node.fields, f)
		if nestedMessage {
			typ := f.Value.Type().Name()
			if f.Value.Kind().String() == "slice" {
				typ = f.Value.Type().Elem().Name()
				fmt.Fprintf(os.Stderr, "slice %s, element type %s\n", f.Value.Type(), f.Value.Type().Elem())
			}
			node.children[typ] = &Node{
				message:  typ,
				fields:   []field{},
				parent:   node,
				children: Nodes{},
			}
			node = node.children[typ]
			depth += 1
		}
	}

	if prevMessage != "" {
		functions.WriteString("  }\n  return b.Build()\n}\n")
	}

	messages.WriteString(buildMessages(0, tree))
}

func buildMessages(depth int, tree Nodes) string {
	messages := new(strings.Builder)
	indent := strings.Repeat("  ", depth)

	for _, node := range tree {
		if depth == 0 {
			messages.WriteString("\n")
		}

		fmt.Fprintf(messages, "%smessage %s {\n", indent, node.message)

		if len(node.children) > 0 {
			messages.WriteString(buildMessages(depth+1, node.children))
		}

		for i, f := range node.fields {
			name := strings.TrimSpace(f.Name)
			typ := f.Value.Type().Name()
			switch typ {
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
				if f.Value.Kind().String() != "struct" {
					typ = f.Value.Kind().String()
				}
			}

			if typ == "slice" {
				typ = "repeated " + f.Value.Type().Elem().Name()
				// }
				// if strings.HasSuffix(name, "[n]") {
				// 	typ = "repeated " + typ
				name = name[:len(name)-3]
			} else if n := strings.Index(name, "[key]"); n >= 0 {
				gocore.Error("Protobuf", fmt.Errorf("map types support for protobuf unverified")).Err()
				typ = "map<" + f.Value.Type().Key().Name() + ", " + f.Value.Type().Elem().Name() + ">"
				name = name[:n]
			}
			fmt.Fprintf(messages, "%s  %s %s = %d;\n", indent, typ, gocore.SnakeCase(name), i+1)
		}

		fmt.Fprintf(messages, "%s}\n", indent)
	}

	return messages.String()
}
