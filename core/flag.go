// Copyright © 2021 The Gomon Project.

package core

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

var (
	// Flags defines and initializes the command line flags
	Flags = flags{
		FlagSet:              flag.FlagSet{},
		version:              false,
		document:             false,
		commandDescription:   "",
		argumentDescriptions: [][2]string{},
		argsMax:              0,
		Port:                 1234,
		Sample:               Sample(15 * time.Second),
	}

	// flagSyntax is a map of flags' names to their command line syntax
	flagSyntax = map[string]string{}

	// logBuf captures error output from Go flag parser
	logBuf = bytes.Buffer{}
)

type (
	flags struct {
		flag.FlagSet
		version              bool
		document             bool
		commandDescription   string
		argumentDescriptions [][2]string
		argsMax              int
		Port                 int
		Sample
	}
)

// Var maps a flag field to its name and description, and adds a brief description
func (f *flags) Var(field interface{}, name, syntax, detail string) {
	flagSyntax[name] = syntax
	switch field := field.(type) {
	case *int:
		f.IntVar(field, name, *field, detail)
	case *uint:
		f.UintVar(field, name, *field, detail)
	case *int64:
		f.Int64Var(field, name, *field, detail)
	case *uint64:
		f.Uint64Var(field, name, *field, detail)
	case *float64:
		f.Float64Var(field, name, *field, detail)
	case *string:
		f.StringVar(field, name, *field, detail)
	case *bool:
		f.BoolVar(field, name, *field, detail)
	case *time.Duration:
		f.DurationVar(field, name, *field, detail)
	default:
		f.FlagSet.Var(field.(flag.Value), name, detail)
	}
}

// init initializes the core command line flags.
func init() {
	log.SetFlags(0)

	Flags.Var(&Flags.version, "version", "[-version]", "Print version information and exit")
	Flags.Var(&Flags.document, "document", "[-document]", "Document the measurements and observations and exit")
	Flags.Var(&Flags.Port, "port", "[-port n]", "Port number for Gomon REST server")

	if Flags.Sample < Sample(time.Second) {
		Flags.Sample = Sample(time.Second)
	}
	Flags.Var(&Flags.Sample, "sample", "[-sample <interval>]",
		"Sample metrics at `interval`, specified in Go time.Duration string format")

	Flags.commandDescription = `Monitors the local host,
	measuring state and usage of:
		• system cpu
		• system memory
		• filesystems
		• I/O devices
		• network interfaces
		• processes
	observing changes to:
		• files
		• logs
		• processes`

	Flags.SetOutput(&logBuf) // capture FlagSet.Parse messages
	Flags.Usage = usage
}

// parse inspects the command line.
func parse(args []string) bool {
	if err := Flags.Parse(args); err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			exitCode = exitError
		}
		return false
	}

	if Flags.NArg() > Flags.argsMax { // too many arguments?
		args := strings.Join(Flags.Args()[Flags.NArg()-Flags.argsMax-1:], " ")
		logBuf.WriteString("invalid arguments: " + args + "\n")
		usage()
		return false
	}

	return true
}

// usage formats the flags Usage message for gomon.
func usage() {
	if !IsTerminal(os.Stderr) && logBuf.Len() > 0 { // if called by go's flag package parser, may have error text
		LogError(errors.New(strings.TrimSpace(logBuf.String()))) // in that case report it
		return
	}

	logBuf.WriteString("NAME:\n  " + commandName)
	logBuf.WriteString("\n\nDESCRIPTION:\n  " + Flags.commandDescription)

	var names []string
	for name := range flagSyntax {
		names = append(names, name)
	}
	sort.Strings(names)
	var flags []string
	for _, name := range names {
		flags = append(flags, flagSyntax[name])
	}
	logBuf.WriteString("\n\nUSAGE:\n  " + commandName + " [-help] " + strings.Join(flags, " "))

	if len(Flags.argumentDescriptions) > 0 {
		for _, args := range Flags.argumentDescriptions {
			logBuf.WriteString(" [" + args[0] + "]")
		}
	}
	logBuf.WriteString(`

VERSION:
  ` + vmmp + `

OPTIONS:
  -help
	Print the help and exit
`)
	Flags.PrintDefaults()

	if len(Flags.argumentDescriptions) > 0 {
		logBuf.WriteString("\nARGUMENTS:\n")
		for _, args := range Flags.argumentDescriptions {
			logBuf.WriteString("  " + args[0] + "\n\t" + args[1] + "\n")
		}
	}
	logBuf.WriteString("\nCopyright © 2021 The Gomon Project.\n")
	fmt.Fprint(os.Stderr, logBuf.String())
}

// IsTerminal reports if the file handle is connected to the terminal.
func IsTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	mode := info.Mode()

	// see https://github.com/golang/go/issues/23123
	if runtime.GOOS == "windows" {
		return mode&os.ModeCharDevice == os.ModeCharDevice
	}

	return mode&(os.ModeDevice|os.ModeCharDevice) == (os.ModeDevice | os.ModeCharDevice)
}

// Sample is a command line flag type.
type Sample time.Duration

// Set is a flag.Value interface method to enable Sample as a command line flag
func (i *Sample) Set(s string) error {
	d, err := time.ParseDuration(s)
	if d <= 0 {
		return errors.New("invalid sample interval")
	}
	*i = Sample(d)
	return err
}

// String is a flag.Value interface method to enable Sample as a command line flag.
func (i Sample) String() string {
	return time.Duration(i).String()
}

// AlignTicker aligns the sample ticking.
func (i Sample) AlignTicker() *time.Ticker {
	d := time.Duration(i)
	t := time.Now()
	<-time.After(d - t.Sub(t.Truncate(d)))
	return time.NewTicker(d)
}

// Regexp is a command line flag type.
type Regexp struct {
	*regexp.Regexp
}

// Set is a flag.Value interface method to enable Regexp as a command line flag.
func (r *Regexp) Set(pattern string) (err error) {
	r.Regexp, err = regexp.Compile(pattern)
	return
}

// String is a flag.Value interface method to enable Regexp as a command line flag.
func (r *Regexp) String() string {
	if r.Regexp == nil {
		return ""
	}
	return r.Regexp.String()
}
