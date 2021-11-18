// Copyright © 2021 The Gomon Project.

package file

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/zosmac/gomon/core"
	"github.com/zosmac/gomon/message"
)

var (
	// obs anchors the observer.
	obs *observer

	// messageChan queues file event observations for periodic reporting.
	messageChan = make(chan *observation, 100)

	// errorChan communicates errors from the listen goroutine.
	errorChan = make(chan error, 10)
)

type (
	// file identifies a file being observed.
	file struct {
		name  string
		isDir bool
	}

	// observer defines the observer's properties.
	observer struct {
		root string
		*handle
		watched map[string]file
	}
)

// Observer starts capture of file update observations.
func Observer() error {
	var err error
	if flags.fileDirectory, err = filepath.Abs(flags.fileDirectory); err != nil {
		return core.NewError("Abs", err)
	}
	if flags.fileDirectory, err = filepath.EvalSymlinks(flags.fileDirectory); err != nil {
		return core.NewError("EvalSymlinks", err)
	}

	h, err := open(flags.fileDirectory)
	if err != nil {
		return core.NewError("open", err)
	}

	obs = &observer{
		root:    flags.fileDirectory,
		handle:  h,
		watched: map[string]file{},
	}

	if err := watchDir("."); err != nil {
		return core.NewError("watch", err)
	}

	core.LogInfo(fmt.Errorf("observing files in %s", flags.fileDirectory))

	go listen()

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

// close stops file observing.
func (obs *observer) close() error {
	obs.handle.close()
	return nil
}

// notify assembles a message and encodes it
func notify(ev fileEvent, f file, oldname string) {
	var msg string
	switch ev {
	case fileCreate:
		msg = f.name
	case fileRename:
		msg = fmt.Sprintf("%s -> %s", oldname, f.name)
	case fileUpdate:
		msg = f.name
	case fileDelete:
		msg = f.name
	}
	messageChan <- &observation{
		Header: message.Observation(time.Now(), sourceFile, ev),
		Id: id{
			Name: f.name,
		},
		Message: msg,
	}
}

// watchDir adds directory to observe
func watchDir(rel string) error {
	err := filepath.WalkDir(filepath.Join(obs.root, rel), func(abs string, entry fs.DirEntry, err error) error {
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) && !errors.Is(err, fs.ErrPermission) {
				core.LogError(fmt.Errorf("WalkDir %v", err))
			}
			return filepath.SkipDir
		}
		base := filepath.Base(abs)
		if base[0] == '.' && len(base) > 1 {
			return filepath.SkipDir // skip hidden directory/file
		}
		if f, err := add(abs, entry.IsDir()); err == nil && !entry.IsDir() {
			if rel != "." { // skip "insert" notifications during initialization
				notify(fileCreate, f, "")
			}
		}
		return nil
	})

	return err
}

// add adds a file to the observer.
func add(abs string, isDir bool) (file, error) {
	for _, f := range obs.watched {
		if abs == f.name {
			return f, nil
		}
	}

	if isDir {
		if err := addDir(abs); err != nil {
			return file{}, err
		}
	}

	rel, _ := filepath.Rel(obs.root, abs)
	f := file{
		name:  abs,
		isDir: isDir,
	}
	obs.watched[rel] = f
	return f, nil
}

// remove removes a file from observation.
func remove(f file) {
	rel, _ := filepath.Rel(obs.root, f.name)
	delete(obs.watched, rel)
	if f.isDir {
		abs := f.name
		for rel, f := range obs.watched {
			f := f
			n, err := filepath.Rel(abs, f.name)
			if err == nil && strings.SplitN(n, string(filepath.Separator), 2)[0] != ".." {
				delete(obs.watched, rel)
				if !f.isDir {
					notify(fileDelete, f, "")
				}
			}
		}
		removeDir(abs)
	} else {
		notify(fileDelete, f, "")
	}
}
