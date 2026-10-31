// Package joblog keeps each job's log as the run daemon writes it and the legacy wtm reads it: a line per line the job printed, its time first, one file per run, rotated past 5 MiB with two backups.
package joblog

import (
	"bytes"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain/run"
)

const (
	maxBytes  = 5 << 20
	backups   = 2
	separator = "  "
	ext       = ".log"
)

// Path returns the log of the job named job in dir, "" for a name no file can
// carry.
func Path(dir, job string) string {
	name := url.PathEscape(job)
	if dir == "" || name == "" || name == "." || name == ".." {
		return ""
	}

	return filepath.Join(dir, name+ext)
}

// A Log is a job's log open for writing; safe for concurrent use.
type Log struct {
	mu   sync.Mutex
	path string
	file *os.File
	size int64
}

// Open opens the log file names, its directory made if needed: emptied, its
// backups gone, unless file.Append.
func Open(file run.LogFile) (*Log, error) {
	path := Path(file.Dir, file.Job)
	if path == "" {
		return nil, errors.New("job " + strconv.Quote(file.Job) + " cannot be named as a log file")
	}
	if err := os.MkdirAll(file.Dir, 0o755); err != nil {
		return nil, err
	}
	flags := os.O_CREATE | os.O_WRONLY | os.O_APPEND
	if !file.Append {
		flags |= os.O_TRUNC
		for rank := 1; rank <= backups; rank++ {
			_ = os.Remove(backup(path, rank))
		}
	}
	opened, err := os.OpenFile(path, flags, 0o644)
	if err != nil {
		return nil, err
	}
	info, err := opened.Stat()
	if err != nil {
		_ = opened.Close()
		return nil, err
	}

	return &Log{path: path, file: opened, size: info.Size()}, nil
}

// Write appends lines, rotating the file once it passed 5 MiB. A log that
// cannot be written stops writing: the job's output matters more than its
// archive.
func (l *Log) Write(lines []run.Line) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil || len(lines) == 0 {
		return
	}

	var buf bytes.Buffer
	for _, line := range lines {
		buf.WriteString(line.At.UTC().Format(time.RFC3339) + separator + line.Text + "\n")
	}
	n, err := l.file.Write(buf.Bytes())
	l.size += int64(n)
	if err == nil && l.size >= maxBytes {
		err = l.rotate()
	}
	if err != nil {
		_ = l.file.Close()
		l.file = nil
	}
}

// rotate moves the file to its first backup, each backup one rank older, the
// oldest dropped, and starts an empty one.
func (l *Log) rotate() error {
	if err := l.file.Close(); err != nil {
		return err
	}
	for rank := backups - 1; rank >= 1; rank-- {
		_ = os.Rename(backup(l.path, rank), backup(l.path, rank+1))
	}
	if err := os.Rename(l.path, backup(l.path, 1)); err != nil {
		return err
	}
	file, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	l.file, l.size = file, 0

	return err
}

// Close closes the log.
func (l *Log) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		_ = l.file.Close()
		l.file = nil
	}
}

// Last returns the last n lines of the log file names, oldest first, read
// from its backup too when the file holds fewer: a line not stamped as this
// package stamps it keeps no time.
func Last(file run.LogFile, n int) []run.Line {
	path := Path(file.Dir, file.Job)
	if path == "" || n <= 0 {
		return nil
	}
	lines := read(path)
	if len(lines) < n {
		lines = append(read(backup(path, 1)), lines...)
	}

	return lines[max(0, len(lines)-n):]
}

func read(path string) []run.Line {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var lines []run.Line
	for text := range strings.Lines(string(data)) {
		lines = append(lines, parse(strings.TrimSuffix(text, "\n")))
	}

	return lines
}

func parse(text string) run.Line {
	stamp, rest, found := strings.Cut(text, separator)
	at, err := time.Parse(time.RFC3339, stamp)
	if !found || err != nil {
		return run.Line{Text: text}
	}

	return run.Line{At: at, Text: rest}
}

func backup(path string, rank int) string {
	return path + "." + strconv.Itoa(rank)
}
