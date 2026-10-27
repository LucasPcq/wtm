package shell

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/statefile"
)

const (
	runLogExt     = ".log"
	runLogPending = "."         // a log being put in place, which pruning leaves alone while it is young
	pendingStale  = time.Minute // a pending log older than this was left by a run that died putting it in place
)

// OpenRunLog opens the log of run, one run of a command, as dir/<run>.log, and
// keeps it exclusively flocked until its Close, so the logs of several runs at
// once never mix. It first removes the other logs of dir no process holds: the
// runs that ended. It returns the log's path and its writer.
func OpenRunLog(dir, run string) (string, io.WriteCloser, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", nil, err
	}
	prune(dir)

	// Locked before it appears under its name: a pruning run never takes a log being opened for ended.
	file, err := os.CreateTemp(dir, runLogPending+run+"-*")
	if err != nil {
		return "", nil, err
	}
	path := filepath.Join(dir, run+runLogExt)
	if err := statefile.Flock(file, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return "", nil, discard(file, err)
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return "", nil, discard(file, err)
	}

	return path, file, nil
}

// prune removes each log of dir whose flock can be taken: no running command writes it; a pending one only once stale.
func prune(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		name := entry.Name()
		pending := strings.HasPrefix(name, runLogPending)
		if entry.IsDir() || !pending && filepath.Ext(name) != runLogExt || pending && !stale(entry) {
			continue
		}
		path := filepath.Join(dir, name)
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		if statefile.Flock(file, syscall.LOCK_EX|syscall.LOCK_NB) == nil {
			_ = os.Remove(path)
		}
		_ = file.Close()
	}
}

func stale(entry os.DirEntry) bool {
	info, err := entry.Info()
	return err == nil && time.Since(info.ModTime()) > pendingStale
}

// discard closes and removes a log that could not be put in place, and returns
// cause with any error of the removal.
func discard(file *os.File, cause error) error {
	_ = file.Close()
	return errors.Join(cause, os.Remove(file.Name()))
}
