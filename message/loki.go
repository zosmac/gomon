// Copyright © 2021-2023 The Gomon Project.

package message

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

var (
	// LokiStreams counts the number of observations streamed to Loki.
	LokiStreams int

	// lokiLabels identifies the unique label types gomon sends to Loki
	lokiLabels = map[string]struct{}{
		"host":     {},
		"platform": {},
		"source":   {},
		"event":    {},
	}

	// lokiStatusRequest queries Loki if it is running.
	lokiStatusRequest = http.Request{
		Method: http.MethodGet,
		URL: &url.URL{
			Scheme: "http",
			Host:   "localhost:3100",
			Path:   "/ready",
		},
	}

	// lokiPushRequest sends observations to Loke.
	lokiPushRequest = http.Request{
		Method: http.MethodPost,
		URL: &url.URL{
			Scheme: "http",
			Host:   "localhost:3100",
			Path:   "/loki/api/v1/push",
		},
		Header: http.Header{"Content-Type": []string{"application/json"}},
	}
)

// lokiTest pings the Loki server to determine if it is ready for accepting file, log, and process observations.
func lokiTest() bool {
	if resp, err := http.DefaultClient.Do(&lokiStatusRequest); err == nil {
		buf, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err == nil && len(buf) >= 5 && string(buf[:5]) == "ready" {
			return true
		}
	}
	return false
}

// lokiFormatter complies with gocore.Formatter function prototype for encoding properties as Loki stream labels and values.
func lokiFormatter(name, tag string, val reflect.Value) any {
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
		gocore.Error("loki message",
			errors.New("property type not recognized"),
			map[string]string{
				"name":  name,
				"value": val.String(),
			},
		).Err()
	}
	return nil
}

// lokiEncode encodes file, log, and process observer observations as Loki streams.
func lokiEncode(ms []Content) bool {
	var s streams
	for _, m := range ms {
		labels := map[string]string{}
		var timestamp, message string
		for _, l := range gocore.Format("", "", reflect.ValueOf(m), lokiFormatter) {
			l := l.(tuple)
			switch l[0] {
			case "timestamp":
				timestamp = l[1]
			case "message":
				message = l[1] + message
			default:
				if _, ok := lokiLabels[l[0]]; ok {
					labels[l[0]] = l[1]
				} else {
					message += " " + l[0] + "=" + l[1]
				}
			}
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
	lokiPushRequest.Body = io.NopCloser(bytes.NewReader(buf))
	if resp, err := http.DefaultClient.Do(&lokiPushRequest); err == nil {
		resp.Body.Close()
		LokiStreams += len(s.Streams)
		return true
	}
	return false
}
