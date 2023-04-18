// Copyright © 2021-2023 The Gomon Project.

package file

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"time"

	"github.com/zosmac/gocore"
	"github.com/zosmac/gomon/message"
)

type (
	// file identifies a file being observed.
	file struct {
		abs   string
		isDir bool
	}

	// observer defines the observer's properties.
	observer struct {
		root string
		*handle
		watched map[string]file
		msgChan chan *observation
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
	}

	if err := watchDir(".", ""); err != nil {
		return gocore.Error("watch", err)
	}

	gocore.LogInfo("observing files", errors.New(flags.fileDirectory))

	if err := observe(); err != nil {
		return gocore.Error("observe", err)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				obs.close()
				return
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
}

// notify assembles a message and encodes it
func notify(ev fileEvent, id, name, oldn string) {
	msg := name
	if ev == fileRename {
		msg += " <- " + oldn
	}
	obs.msgChan <- &observation{
		Header: message.Observation(time.Now(), ev),
		Id: Id{
			Name:    name,
			EventID: id,
		},
		Message: msg,
	}
}

// watchDir adds directory to observe
func watchDir(rel, id string) error {
	if err := filepath.WalkDir(filepath.Join(obs.root, rel), func(abs string, entry fs.DirEntry, err error) error {
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) && !errors.Is(err, fs.ErrPermission) {
				gocore.LogError("WalkDir", err)
			}
			return filepath.SkipDir
		}
		base := filepath.Base(abs)
		if base[0] == '.' && len(base) > 1 {
			return filepath.SkipDir // skip hidden directory/file
		}
		if f, err := add(abs, entry.IsDir()); err == nil && !entry.IsDir() {
			if rel != "." { // skip "insert" notifications during initialization
				notify(fileCreate, id, f.abs, "")
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
		if abs == f.abs {
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
		abs:   abs,
		isDir: isDir,
	}
	obs.watched[rel] = f
	return f, nil
}

// remove removes a file from observation.
func remove(f file, id string) {
	rel, _ := filepath.Rel(obs.root, f.abs)
	delete(obs.watched, rel)
	if f.isDir {
		abs := f.abs
		for rel, f := range obs.watched {
			f := f
			if _, err := gocore.Subdir(abs, f.abs); err == nil {
				delete(obs.watched, rel)
				if !f.isDir {
					notify(fileDelete, id, f.abs, "")
				}
			}
		}
		removeDir(abs)
	} else {
		notify(fileDelete, id, f.abs, "")
	}
}
