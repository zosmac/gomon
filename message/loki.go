// Copyright © 2021-2023 The Gomon Project.

package message

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/zosmac/gocore"
)

type (
	// tuple type defines a string pair for formatting Loki log entry labels and values.
	tuple [2]string

	// stream type defined for generating Loki log entries.
	stream struct {
		Stream map[string]string `json:"stream"`
		Values []tuple           `json:"values"`
	}

	// streams type defined for generating Loki log entries.
	streams struct {
		Streams []stream `json:"streams"`
	}
)

// lokiTest pings for the Loki server to determine if it is ready for accepting file, log, and process observer observations.
func lokiTest() bool {
	if resp, err := http.DefaultClient.Get("http://localhost:3100/ready"); err == nil {
		buf, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err == nil && len(buf) >= 5 && string(buf[:5]) == "ready" {
			return true
		}
	}
	return false
}

// lokiFormatter complies with gocore.Formatter function prototype for encoding properties as Loki stream labels and values.
func lokiFormatter(name, tag string, val reflect.Value) interface{} {
	if strings.HasPrefix(tag, "property") {
		if val.Kind() == reflect.String {
			return tuple{name, val.String()}
		} else if val.Type().ConvertibleTo(reflect.TypeOf(time.Time{})) {
			t := val.Convert(reflect.TypeOf(time.Time{})).Interface().(time.Time)
			if name == "timestamp" {
				return tuple{name, strconv.FormatInt(t.UnixNano(), 10)}
			} else {
				return tuple{name, t.Format("2006-01-02 15:04:05.999")}
			}
		} else if s, ok := val.Interface().(fmt.Stringer); ok {
			return tuple{name, s.String()}
		} else {
			switch val.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				return tuple{name, strconv.FormatInt(val.Int(), 10)}
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				return tuple{name, strconv.FormatUint(val.Uint(), 10)}
			case reflect.Float32, reflect.Float64:
				return tuple{name, strconv.FormatFloat(val.Float(), 'e', -1, 64)}
			}
		}
		gocore.LogError(
			"lokiMessage property type not recognized",
			fmt.Errorf("%s %v", name, val.Interface()),
		)
	}
	return nil
}

// lokiEncode encodes file, log, and process observer observations as Loki streams.
func lokiEncode(os []Content) bool {
	var s streams
	for _, o := range os {
		ls := map[string]string{}
		for _, l := range gocore.Format("", "", reflect.ValueOf(o), lokiFormatter) {
			l := l.(tuple)
			ls[l[0]] = l[1]
		}

		labels := map[string]string{}

		timestamp := ls["timestamp"]
		message := ls["message"]
		labels["source"] = ls["source"]
		labels["event"] = ls["event"]
		if labels["source"] != "file" {
			labels["name"] = ls["id_name"] // file name already in message text
		}

		s.Streams = append(s.Streams, stream{
			Stream: labels,
			Values: []tuple{
				{
					timestamp,
					message,
				},
			},
		})
	}

	buf, _ := json.Marshal(s)
	if resp, err := http.DefaultClient.Post(
		"http://localhost:3100/loki/api/v1/push",
		"application/json",
		bytes.NewReader(buf),
	); err == nil {
		resp.Body.Close()
		return true
	}
	return false
}
