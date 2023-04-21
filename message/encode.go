// Copyright © 2021-2023 The Gomon Project.

package message

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zosmac/gocore"
)

type (
	// writer wraps os.Stdout.
	writer struct{}
)

var (
	// measureChan sends measurements for encoding.
	measureChan = make(chan []Content, 100)

	// observeChan sends observations for encoding.
	observeChan = make(chan []Content, 100)

	// jsonEncoder encodes Content as JSON.
	jsonEncoder *json.Encoder

	// cache a time series stream of messages.
	cache []Content
)

// Write enables writer to conform to io.Writer, indirection allows stdout destination to rotate.
func (writer) Write(buf []byte) (int, error) { return os.Stdout.Write(buf) }

// Encoder configures the message encoder. Its Encode function marshals objects.
func Encoder(ctx context.Context) error {
	info, err := os.Stdout.Stat()
	if err != nil {
		return gocore.Error("Stat", err)
	}

	if info.Mode().IsRegular() {
		if path, err := gocore.FdPath(int(os.Stdout.Fd())); err == nil {
			if strings.ToLower(filepath.Ext(path)) == ".json" {
				// if writing json to file, must cache all objects to produce valid json as array
				if flags.rotate.String() == "0s" {
					flags.rotate.Set(time.Duration(0x7FFFFFFFFFFFFFFF).String())
				}
			}
		}
	} else {
		flags.rotate.Set("0s") // rotate interval is meaningless for a non-file destination
	}

	jsonEncoder = json.NewEncoder(writer{})
	jsonEncoder.SetEscapeHTML(false)
	if flags.pretty {
		jsonEncoder.SetIndent("", "  ")
	}

	go encode(ctx)

	return nil
}

// Measure sends measurements to encode.
func Measure(ms []Content) error {
	measureChan <- ms
	return nil
}

// Observe sends observations to encode.
func Observe(os []Content) error {
	observeChan <- os
	return nil
}

// encode runs as a goroutine that receives messages and encodes or caches them.
func encode(ctx context.Context) {
	var timer, ticker <-chan time.Time
	if flags.rotate.interval > 0 {
		tm := time.Now().UTC().Truncate(flags.rotate.interval)
		// synchronize rotation to occur at the 'top' of the interval (day, hour, minute)
		timer = time.NewTimer(time.Until(tm.Add(flags.rotate.interval))).C
	}

	loki := lokiTest()
	lokiTick := time.NewTicker(5 * time.Second).C
	for {
		select {
		case ms := <-measureChan:
			fileEncode(ms)
		case ms := <-observeChan:
			if loki {
				loki = lokiEncode(ms)
			}
			if !loki {
				fileEncode(ms)
			}
		case <-lokiTick:
			if !loki {
				loki = lokiTest()
			}
		case t := <-timer: // first rotate on 'top' of interval
			ticker = time.NewTicker(flags.rotate.interval).C // start subsequent ticking
			Rotate(t)
		case t := <-ticker:
			Rotate(t)
		case <-ctx.Done():
			Close()
			gocore.LogInfo("Encoder", ctx.Err())
			return
		}
	}
}

// fileEncode encodes the content for writing to file.
func fileEncode(ms []Content) {
	if flags.rotate.interval == 0 {
		for _, m := range ms {
			jsonEncoder.Encode(m)
		}
	} else {
		// gather json objects written to file into a single array object
		cache = append(cache, ms...)
	}
}

// Close completes the jsonEncoder's operation.
func Close() {
	Rotate(time.Now())
}

// Rotate obtains lock and calls encoder's rotate method.
func Rotate(t time.Time) {
	if len(cache) > 0 {
		var i any
		if len(cache) == 1 {
			i = cache[0]
		} else {
			i = cache
		}
		err := jsonEncoder.Encode(i)
		cache = nil
		if err != nil {
			gocore.LogError("Encode", err)
			return
		}
	}

	timestamp := t.UTC().Format(flags.rotate.format)

	info, err := os.Stdout.Stat()
	if err != nil {
		gocore.LogError("Stat", err)
		return
	}

	if !info.Mode().IsRegular() {
		return
	}

	oldpath, err := gocore.FdPath(int(os.Stdout.Fd()))
	if err != nil {
		gocore.LogError("FdPath", err)
		return
	}

	ext := filepath.Ext(oldpath)
	base := strings.TrimSuffix(filepath.Base(oldpath), ext)
	newpath := filepath.Join(filepath.Dir(oldpath), base+"-"+timestamp+ext)

	if err := os.Rename(oldpath, newpath); err != nil {
		gocore.LogError("Rename", err)
		return
	}

	sout, err := os.Create(oldpath)
	if err != nil {
		gocore.LogError("Create", err)
		return
	}
	chown(sout, info)

	old := os.Stdout
	os.Stdout = sout
	old.Close()
}
