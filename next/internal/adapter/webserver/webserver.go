// Package webserver keeps the record of the one web server of this machine: the lock its instance holds while it serves, web.json, where it listens, its version and its token, and the page that opens it, both readable by their user alone.
package webserver

import (
	"context"
	"encoding/json"
	"errors"
	"html"
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
	launchName    = "web.html"
	// recordPerm keeps the token from the other users of the machine.
	recordPerm os.FileMode = 0o600
)

// errNoDir refuses to keep the server's files when the user's config directory is unknown, rather than in the working directory.
var errNoDir = errors.New("the user's config directory is unknown: no web server can be recorded")

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
	path, err := f.path(lockName)
	if err != nil {
		return nil, err
	}
	file, err := statefile.TryLock(path, domain.LockExclusive)
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
	_, err = f.write(ctx, recordName, data)
	return err
}

// Read returns the record of the last server started; it fails with an error wrapping fs.ErrNotExist when none was.
func (f Files) Read() (Record, error) {
	path, err := f.path(recordName)
	if err != nil {
		return Record{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Record{}, err
	}

	var record Record
	err = json.Unmarshal(data, &record)
	return record, err
}

// Launch writes the page that sends the browser to address, readable by its user alone, and returns its path: a browser opened on it never carries the token of address in its command line, which any user of the machine may read.
func (f Files) Launch(ctx context.Context, address string) (string, error) {
	escaped := html.EscapeString(address)
	page := `<!doctype html><meta charset="utf-8"><meta http-equiv="refresh" content="0;url=` + escaped + `"><title>wtm</title><a href="` + escaped + `">Open wtm</a>` + "\n"
	return f.write(ctx, launchName, []byte(page))
}

// write replaces the file name with data, readable by its user alone, and returns its path.
func (f Files) write(ctx context.Context, name string, data []byte) (string, error) {
	path, err := f.path(name)
	if err != nil {
		return "", err
	}
	_, err = statefile.Update(ctx, statefile.UpdateParams{
		Path: path,
		Perm: recordPerm,
		Mutate: func([]byte) (statefile.Change, error) {
			return statefile.Write{Data: data}, nil
		},
	})
	if err != nil {
		return "", err
	}
	// a file left by an older wtm with wider permissions keeps them through an update
	return path, os.Chmod(path, recordPerm)
}

func (f Files) path(name string) (string, error) {
	if f.Dir == "" {
		return "", errNoDir
	}
	return filepath.Join(f.Dir, name), nil
}
