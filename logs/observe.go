// Copyright © 2021-2023 The Gomon Project.

package logs

import (
	"bufio"
	"context"
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
	levelMap = map[string]LogEvent{
		"emerg":      LevelFatal, // Apache
		"emergency":  LevelFatal, // syslog
		"fatal":      LevelFatal,
		"fault":      LevelFatal, // macOS
		"panic":      LevelFatal, // syslog, Postgres
		"alert":      LevelError, // syslog, Apache
		"crash":      LevelError, // RabbitMQ
		"crit":       LevelError, // syslog, Apache
		"critical":   LevelError, // syslog, RabbitMQ
		"err":        LevelError, // syslog, Consul, Vault
		"error":      LevelError,
		"supervisor": LevelWarn, // RabbitMQ
		"warn":       LevelWarn,
		"warning":    LevelWarn, // syslog, Postgres
		"info":       LevelInfo,
		"":           LevelInfo, // treat unknown as info
		"log":        LevelInfo, // Postgres
		"notice":     LevelInfo, // syslog, Postgres, Apache, macOS
		"statement":  LevelInfo, // Postgres
		"debug":      LevelDebug,
		"debug1":     LevelDebug, // Postgres
		"debug2":     LevelDebug, // Postgres
		"debug3":     LevelDebug, // Postgres
		"debug4":     LevelDebug, // Postgres
		"debug5":     LevelDebug, // Postgres
		"default":    LevelDebug, // macOS
		"trace":      LevelTrace,
		"trace1":     LevelTrace, // Apache
		"trace2":     LevelTrace, // Apache
		"trace3":     LevelTrace, // Apache
		"trace4":     LevelTrace, // Apache
		"trace5":     LevelTrace, // Apache
		"trace6":     LevelTrace, // Apache
		"trace7":     LevelTrace, // Apache
		"trace8":     LevelTrace, // Apache
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

	for sc.Scan() {
		match := regex.FindStringSubmatch(sc.Text())
		if len(match) == 0 || match[0] == "" {
			continue
		}

		if runtime.GOOS == "linux" {
			level := levelMap[strings.ToLower(match[groups[groupLevel]])]
			if !logEvents.IsValid(level) || logEvents.Index(level) < logEvents.Index(Flags.LogEvent) {
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

		queue(groups, format, match)
	}
}

func queue(groups map[string]int, format string, match []string) {
	t, _ := time.Parse(format, match[groups[groupTimestamp]])
	pid, _ := strconv.Atoi(match[groups[groupPid]])
	sender := match[groups[groupSender]]
	if cg, ok := groups[groupSubCat]; ok {
		sender = match[cg] + ":" + sender
	}

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
