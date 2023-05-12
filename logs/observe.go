// Copyright © 2021-2023 The Gomon Project.

package logs

import (
	"bufio"
	"context"
	"fmt"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/zosmac/gocore"
	"github.com/zosmac/gomon/message"
)

const (
	// log record regular expressions capture group names.
	groupTimestamp = "timestamp"
	groupUtc       = "utc"
	groupTzoffset  = "tzoffset"
	groupTimezone  = "timezone"
	groupLevel     = "level"
	groupHost      = "host"
	groupProcess   = "process"
	groupPid       = "pid"
	groupThread    = "thread"
	groupSender    = "sender"
	groupSubCat    = "subcat"
	groupMessage   = "message"
)

var (
	// messageChan queues log messages for encoding.
	messageChan = make(chan *observation, 100)

	// levelMap maps various applications' log levels to a common set fatal/error/warn/info/debug/trace.
	levelMap = map[string]logEvent{
		"emerg":      eventFatal, // Apache
		"emergency":  eventFatal, // syslog
		"fatal":      eventFatal,
		"fault":      eventFatal, // macOS
		"panic":      eventFatal, // syslog, Postgres
		"alert":      eventError, // syslog, Apache
		"crash":      eventError, // RabbitMQ
		"crit":       eventError, // syslog, Apache
		"critical":   eventError, // syslog, RabbitMQ
		"err":        eventError, // syslog, Consul, Vault
		"error":      eventError,
		"supervisor": eventWarn, // RabbitMQ
		"warn":       eventWarn,
		"warning":    eventWarn, // syslog, Postgres
		"info":       eventInfo,
		"":           eventInfo, // treat unknown as info
		"log":        eventInfo, // Postgres
		"notice":     eventInfo, // syslog, Postgres, Apache, macOS
		"statement":  eventInfo, // Postgres
		"debug":      eventDebug,
		"debug1":     eventDebug, // Postgres
		"debug2":     eventDebug, // Postgres
		"debug3":     eventDebug, // Postgres
		"debug4":     eventDebug, // Postgres
		"debug5":     eventDebug, // Postgres
		"default":    eventDebug, // macOS
		"trace":      eventTrace,
		"trace1":     eventTrace, // Apache
		"trace2":     eventTrace, // Apache
		"trace3":     eventTrace, // Apache
		"trace4":     eventTrace, // Apache
		"trace5":     eventTrace, // Apache
		"trace6":     eventTrace, // Apache
		"trace7":     eventTrace, // Apache
		"trace8":     eventTrace, // Apache
	}
)

// Observer starts the log monitor.
func Observer(ctx context.Context) error {
	if err := open(); err != nil {
		return gocore.Error("open", err)
	}

	if err := observe(ctx); err != nil {
		return gocore.Error("observe", err)
	}

	go func() {
		defer close()
		for {
			select {
			case <-ctx.Done():
				return
			case obs, ok := <-messageChan:
				if !ok {
					return
				}
				message.Observations([]message.Content{obs})
			}
		}
	}()

	return nil
}

func parseLog(sc *bufio.Scanner, regex *regexp.Regexp, format string) {
	groups := func() map[string]int {
		g := map[string]int{}
		for _, name := range regex.SubexpNames() {
			g[name] = regex.SubexpIndex(name)
		}
		return g
	}()

	dedups := [][]string{nil, nil, nil}

	for sc.Scan() {
		match := regex.FindStringSubmatch(sc.Text())
		if len(match) == 0 || match[0] == "" {
			continue
		}

		if runtime.GOOS == "linux" {
			level := levelMap[strings.ToLower(match[groups[groupLevel]])]
			if !logEvents.IsValid(level) || logEvents.Index(level) < logEvents.Index(Flags.logEvent) {
				continue
			}

			match[groups[groupTimestamp]] = strings.Replace(match[groups[groupTimestamp]], "/", "-", 2) // replace '/' with '-' in date
			match[groups[groupTimestamp]] = strings.Replace(match[groups[groupTimestamp]], "T", " ", 1) // replace 'T' with ' ' between date and time

			if match[groups[groupUtc]] == "Z" || match[groups[groupTzoffset]] != "" {
				format = "2006-01-02 15:04:05Z07:00"
			} else if match[groups[groupTimezone]] != "" {
				format = "2006-01-02 15:04:05 MST"
			}
		}

		queue(groups, format, match, dedups)
	}
}

func queue(groups map[string]int, format string, match []string, dedups [][]string) {
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			buf = buf[:n]
			gocore.Error("queue panic", nil, map[string]string{
				"panic":      fmt.Sprint(r),
				"stacktrace": string(buf),
			}).Err()
		}
	}()

loop:
	for _, dedup := range dedups {
		if len(match) != len(dedup) {
			continue
		}
		for i := range match {
			if i == 0 || i == groups[groupTimestamp] {
				continue
			}
			if match[i] != dedup[i] {
				continue loop
			}
		}
		return
	}
	for i := len(dedups) - 1; i != 0; i-- {
		dedups[i] = dedups[i-1]
	}
	dedups[0] = match

	pid, _ := strconv.Atoi(match[groups[groupPid]])
	sender := match[groups[groupSender]]
	if cg, ok := groups[groupSubCat]; ok {
		sender = match[cg] + ":" + sender
	}

	t, _ := time.Parse(format, match[groups[groupTimestamp]])
	messageChan <- &observation{
		Header: message.Observation(t, levelMap[strings.ToLower(match[groups[groupLevel]])]),
		Id: Id{
			Name:   match[groups[groupProcess]],
			Pid:    pid,
			Sender: sender,
		},
		Message: match[groups[groupMessage]],
	}
}
