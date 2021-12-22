// Copyright © 2021 The Gomon Project.

package log

import (
	"github.com/zosmac/gomon/core"
	"github.com/zosmac/gomon/message"
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

type (
	// captureGroup is the name of a reqular expression capture group.
	captureGroup string
)

// Observer starts capture of log entries.
func Observer() error {
	if err := open(); err != nil {
		return core.Error("open", err)
	}

	go observe()

	go func() {
		for {
			select {
			case err := <-errorChan:
				core.LogError(err)
			case obs := <-messageChan:
				message.Encode([]message.Content{obs})
			}
		}
	}()

	return nil
}
