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

	// errorChan communicates errors from the observe goroutine.
	errorChan = make(chan error, 10)

	// levelMap maps various applications' log levels to a common set fatal/error/warn/info/debug/trace.
	levelMap = map[string]logLevel{
		"emerg":      levelFatal, // Apache
		"emergency":  levelFatal, // syslog
		"fatal":      levelFatal,
		"fault":      levelFatal, // macOS
		"panic":      levelFatal, // syslog, Postgres
		"alert":      levelError, // syslog, Apache
		"crash":      levelError, // RabbitMQ
		"crit":       levelError, // syslog, Apache
		"critical":   levelError, // syslog, RabbitMQ
		"err":        levelError, // syslog, Consul, Vault
		"error":      levelError,
		"supervisor": levelWarn, // RabbitMQ
		"warn":       levelWarn,
		"warning":    levelWarn, // syslog, Postgres
		"info":       levelInfo,
		"":           levelInfo, // treat unknown as info
		"log":        levelInfo, // Postgres
		"notice":     levelInfo, // syslog, Postgres, Apache, macOS
		"statement":  levelInfo, // Postgres
		"debug":      levelDebug,
		"debug1":     levelDebug, // Postgres
		"debug2":     levelDebug, // Postgres
		"debug3":     levelDebug, // Postgres
		"debug4":     levelDebug, // Postgres
		"debug5":     levelDebug, // Postgres
		"default":    levelDebug, // macOS
		"trace":      levelTrace,
		"trace1":     levelTrace, // Apache
		"trace2":     levelTrace, // Apache
		"trace3":     levelTrace, // Apache
		"trace4":     levelTrace, // Apache
		"trace5":     levelTrace, // Apache
		"trace6":     levelTrace, // Apache
		"trace7":     levelTrace, // Apache
		"trace8":     levelTrace, // Apache
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
		for {
			select {
			case <-ctx.Done():
				return
			case err, ok := <-errorChan:
				if !ok {
					return
				}
				gocore.LogError(err)
			case obs, ok := <-messageChan:
				if !ok {
					return
				}
				message.Encode([]message.Content{obs})
			}
		}
	}()

	return nil
}

func parseLog(ctx context.Context, sc *bufio.Scanner, regex *regexp.Regexp, format string) {
	groups := func() map[string]int {
		g := map[string]int{}
		for _, name := range regex.SubexpNames() {
			g[name] = regex.SubexpIndex(name)
		}
		return g
	}()

	readyChan := make(chan struct{})
	go func() {
		for sc.Scan() {
			readyChan <- struct{}{}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-readyChan:
		}
		match := regex.FindStringSubmatch(sc.Text())
		if len(match) == 0 || match[0] == "" {
			continue
		}

		if runtime.GOOS == "linux" {
			level := levelMap[strings.ToLower(match[groups[groupLevel]])]
			if !logLevels.IsValid(level) || logLevels.Index(level) < logLevels.Index(flags.logLevel) {
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
