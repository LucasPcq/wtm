// Package webserver keeps the record of the one web server of this machine: the lock its instance holds while it serves, and web.json, where it listens, its version and its token, readable by its user alone.
package webserver

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"syscall"

	"github.com/LucasPcq/wtm/next/internal/adapter/statefile"
	"github.com/LucasPcq/wtm/next/internal/domain"
)

const (
	globalDirName = "wtm"
	recordName    = "web.json"
	lockName      = "web.lock"
	// recordPerm keeps the token from the other users of the machine.
	recordPerm os.FileMode = 0o600
)

// ErrRunning is the error of [Files.Lock] when another process holds the lock: a server already serves.
var ErrRunning = errors.New("a web server already runs on this machine")

// A Record is what a running server says of itself: its process, the port it listens on at 127.0.0.1, the version of its binary and the token every request to it carries.
type Record struct {
	PID     int    `json:"pid"`
	Port    int    `json:"port"`
	Version string `json:"version"`
	Token   string `json:"token"`
}

// Files are the server's files in Dir.
type Files struct {
	Dir string
}

// Dir returns where the server keeps its files, beside repos.json under the user's config directory, or "" when that directory is unknown.
func Dir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}

	return filepath.Join(dir, globalDirName)
}

// Lock takes the lock of the one server of the machine, without waiting, and returns its release; it fails with [ErrRunning] while another process holds it.
func (f Files) Lock() (func(), error) {
	file, err := statefile.TryLock(filepath.Join(f.Dir, lockName), domain.LockExclusive)
	if errors.Is(err, statefile.ErrBusy) {
		return nil, ErrRunning
	}
	if err != nil {
		return nil, err
	}

	return func() {
		_ = statefile.Flock(file, syscall.LOCK_UN)
		_ = file.Close()
	}, nil
}

// Write records record, readable by its user alone.
func (f Files) Write(ctx context.Context, record Record) error {
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}

	path := filepath.Join(f.Dir, recordName)
	_, err = statefile.Update(ctx, statefile.UpdateParams{
		Path: path,
		Perm: recordPerm,
		Mutate: func([]byte) (statefile.Change, error) {
			return statefile.Write{Data: data}, nil
		},
	})
	if err != nil {
		return err
	}
	// a record left by an older wtm with wider permissions keeps them through an update
	return os.Chmod(path, recordPerm)
}

// Read returns the record of the last server started; it fails with an error wrapping fs.ErrNotExist when none was.
func (f Files) Read() (Record, error) {
	data, err := os.ReadFile(filepath.Join(f.Dir, recordName))
	if err != nil {
		return Record{}, err
	}

	var record Record
	err = json.Unmarshal(data, &record)
	return record, err
}
