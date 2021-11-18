// Copyright © 2021 The Gomon Project.

package message

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zosmac/gomon/core"
)

var (
	jsonEncoder *json.Encoder

	messageChan = make(chan []Content, 100)

	// cache a time series stream of messages.
	cache []Content
)

type (
	// writer wraps os.Stdout.
	writer struct{}
)

// Write enables writer to conform to io.Writer, indirection allows stdout destination to rotate.
func (writer) Write(buf []byte) (int, error) { return os.Stdout.Write(buf) }

// Encoder configures the message encoder. Its Encode function marshals objects.
func Encoder() error {
	info, err := os.Stdout.Stat()
	if err != nil {
		return core.NewError("Stat", err)
	}

	if info.Mode().IsRegular() {
		if path, err := core.FdPath(int(os.Stdout.Fd())); err == nil {
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

	go encode()

	core.Register(func() { Close() })

	return nil
}

// Encode calls the configured encoder function.
func Encode(ms []Content) error {
	messageChan <- ms
	return nil
}

// encode runs as a goroutine that receives messages and encodes or caches them.
func encode() {
	var timer <-chan time.Time
	if flags.rotate.interval > 0 {
		tm := time.Now().UTC().Truncate(flags.rotate.interval)
		// synchronize rotation to occur at the 'top' of the interval (day, hour, minute)
		timer = time.NewTimer(time.Until(tm.Add(flags.rotate.interval))).C
	}

	loki := lokiTest()
	for {
		select {
		case ms := <-messageChan:
			if loki {
				loki = lokiEncode(ms)
			}
			if loki {
				continue
			}

			if flags.rotate.interval == 0 {
				for _, m := range ms {
					jsonEncoder.Encode(m)
				}
			} else {
				// gather json objects written to file into a single array object
				cache = append(cache, ms...)
			}
		case t := <-timer:
			timer = time.NewTicker(flags.rotate.interval).C
			Rotate(t)
		}

		loki = lokiTest()
	}
}

// Close completes the jsonEncoder's operation.
func Close() error {
	return Rotate(time.Now())
}

// Rotate obtains lock and calls encoder's rotate method.
func Rotate(t time.Time) error {
	if len(cache) > 0 {
		var i interface{}
		if len(cache) == 1 {
			i = cache[0]
		} else {
			i = cache
		}
		err := jsonEncoder.Encode(i)
		cache = nil
		if err != nil {
			return err
		}
	}

	timestamp := t.UTC().Format(flags.rotate.format)

	info, err := os.Stdout.Stat()
	if err != nil {
		core.LogError(err)
		return err
	}

	if !info.Mode().IsRegular() {
		return nil
	}

	oldpath, err := core.FdPath(int(os.Stdout.Fd()))
	if err != nil {
		core.LogError(err)
		return err
	}

	ext := filepath.Ext(oldpath)
	base := strings.TrimSuffix(filepath.Base(oldpath), ext)
	newpath := filepath.Join(filepath.Dir(oldpath), base+"-"+timestamp+ext)

	if err := os.Rename(oldpath, newpath); err != nil {
		core.LogError(err)
		return err
	}

	sout, err := os.Create(oldpath)
	if err != nil {
		return core.NewError("Create", err)
	}
	chown(sout, info)

	old := os.Stdout
	os.Stdout = sout
	old.Close()

	return nil
}
