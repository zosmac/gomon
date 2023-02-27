// Copyright © 2021-2023 The Gomon Project.

package file

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"time"

	"github.com/zosmac/gocore"
	"github.com/zosmac/gomon/message"
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
		msgChan chan *observation
		errChan chan error
	}
)

var (
	// obs anchors the observer.
	obs *observer
)

// Observer starts capture of file update observations.
func Observer(ctx context.Context) error {
	var err error
	if flags.fileDirectory, err = filepath.Abs(flags.fileDirectory); err != nil {
		return gocore.Error("Abs", err)
	}
	if flags.fileDirectory, err = filepath.EvalSymlinks(flags.fileDirectory); err != nil {
		return gocore.Error("EvalSymlinks", err)
	}

	h, err := open(flags.fileDirectory)
	if err != nil {
		return gocore.Error("open", err)
	}

	obs = &observer{
		root:    flags.fileDirectory,
		handle:  h,
		watched: map[string]file{},
		msgChan: make(chan *observation, 100),
		errChan: make(chan error, 10),
	}

	if err := watchDir("."); err != nil {
		return gocore.Error("watch", err)
	}

	gocore.LogInfo(fmt.Errorf("observing files in %q", flags.fileDirectory))

	if err := observe(ctx); err != nil {
		return gocore.Error("observe", err)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				obs.close()
				return
			case err, ok := <-obs.errChan:
				if !ok {
					return
				}
				gocore.LogError(err)
			case msg, ok := <-obs.msgChan:
				if !ok {
					return
				}
				message.Encode([]message.Content{msg})
			}
		}
	}()

	return nil
}

// close stops file observing.
func (obs *observer) close() {
	obs.handle.close()
	close(obs.msgChan)
	close(obs.errChan)
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
	obs.msgChan <- &observation{
		Header: message.Observation(time.Now(), ev),
		Id: Id{
			Name: f.name,
		},
		Message: msg,
	}
}

// watchDir adds directory to observe
func watchDir(rel string) error {
	if err := filepath.WalkDir(filepath.Join(obs.root, rel), func(abs string, entry fs.DirEntry, err error) error {
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) && !errors.Is(err, fs.ErrPermission) {
				gocore.LogError(gocore.Error("WalkDir", err))
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
	}); err != nil {
		return gocore.Error("watchDir", err)
	}

	return nil
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
			if n, err := filepath.Rel(abs, f.name); err == nil && (len(n) < 2 || n[:2] != "..") {
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
