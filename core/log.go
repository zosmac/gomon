// Copyright © 2021 The Gomon Project.

package core

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

var (
	logLevel = func() int {
		switch strings.ToUpper(os.Getenv("LOG_LEVEL")) {
		case "TRACE":
			return levelTrace
		case "DEBUG":
			return levelDebug
		case "WARN":
			return levelWarn
		case "ERROR":
			return levelError
		}
		return levelInfo
	}()

	// exitCode: 0 SUCCESS, 1 WARN, 2 ERROR
	exitCode = exitSuccess
)

const (
	levelTrace = iota - 2
	levelDebug
	levelInfo // default
	levelWarn
	levelError
)

const (
	exitSuccess = iota
	exitWarn
	exitError
)

type (
	// Err custom logging error type
	Err struct {
		s   string
		err error
	}
)

// Error method to comply with error interface
func (err *Err) Error() string {
	return err.s
}

// Unwrap method to comply with error interface
func (err *Err) Unwrap() error {
	return err.err
}

// NewError formats an error with function name, errno number, and error message, with location
// details for initial error, preserving the initial logged error for percolation.
func Error(name string, err error) *Err {
	return logMessage(2, name, err)
}

// Unsupported reports that a specific OS does not support a function
func Unsupported() error {
	return fmt.Errorf("%s unsupported", runtime.GOOS)
}

// logWrite writes a log message to the log destination.
func logWrite(level string, err error) {
	if msg := logMessage(3, "", err); msg != nil {
		log.Printf("%s %-5s %s", time.Now().Format(TimeFormat), level, msg)
	}
}

// logMessage formats a log message with where, what, how, which, who, why of an error.
func logMessage(depth int, name string, err error) *Err {
	if err == nil {
		return nil
	}
	e := &Err{}
	if errors.As(err, &e) {
		return e // percolate original Err
	}
	c := append([]string{executable}, os.Args[1:]...)
	_, n, l, _ := runtime.Caller(depth)
	n, _ = filepath.Rel(srcpath, n)
	var s string
	if u, err := user.Current(); err == nil {
		s = u.Username
	}
	msg := fmt.Sprintf("%s [%s %s %s] [%s] - [%s:%d] ", c, commandName, vmmp, buildDate, s, n, l)
	if name != "" {
		msg += name + ": "
	}
	var errno syscall.Errno
	if errors.As(err, &errno) {
		msg += fmt.Sprintf("errno %d: ", errno)
	}
	return &Err{
		s:   msg + err.Error(),
		err: err,
	}
}

// LogTrace log trace message.
func LogTrace(err error) {
	if logLevel <= levelTrace {
		logWrite("TRACE", err)
	}
}

// LogDebug log debug message.
func LogDebug(err error) {
	if logLevel <= levelDebug {
		logWrite("DEBUG", err)
	}
}

// LogInfo log info message (default logging level).
func LogInfo(err error) {
	if logLevel <= levelInfo {
		logWrite("INFO", err)
	}
}

// LogWarn log warning message, setting exit code to WARN.
func LogWarn(err error) {
	if logLevel <= levelWarn {
		logWrite("WARN", err)
	}
	if exitCode < exitWarn {
		exitCode = exitWarn
	}
}

// LogError log error message, setting exit code to ERROR.
func LogError(err error) {
	if logLevel <= levelError {
		logWrite("ERROR", err)
	}
	if exitCode < exitError {
		exitCode = exitError
	}
}
