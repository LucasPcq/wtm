// Package joblog keeps each job's log as the run daemon writes it and the legacy wtm reads it: a line per line the job printed, its time first, one file per run, rotated past 5 MiB with two backups.
package joblog

import (
	"bytes"
	"errors"
	"io"
	"math"
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
	// tailRead is how much of the end of a log Last reads first: a few lines never cost the whole file.
	tailRead = 64 << 10
	// A job's output may hold what its user would not share: its logs are its user's alone.
	dirMode  = 0o700
	fileMode = 0o600
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
// backups gone, unless file.Append. The file is its user's alone, a legacy
// wtm's narrowed as it opens; a directory made for it too, and one a legacy
// wtm made, which holds this job's log already. Any other directory a start
// names keeps its mode: it may be shared on purpose.
func Open(file run.LogFile) (*Log, error) {
	path := Path(file.Dir, file.Job)
	if path == "" {
		return nil, errors.New("job " + strconv.Quote(file.Job) + " cannot be named as a log file")
	}
	if _, err := os.Stat(path); err == nil {
		if err := os.Chmod(file.Dir, dirMode); err != nil {
			return nil, err
		}
	}
	if err := os.MkdirAll(file.Dir, dirMode); err != nil {
		return nil, err
	}
	flags := os.O_CREATE | os.O_WRONLY | os.O_APPEND
	if !file.Append {
		flags |= os.O_TRUNC
		for rank := 1; rank <= backups; rank++ {
			_ = os.Remove(backup(path, rank))
		}
	}
	opened, err := os.OpenFile(path, flags, fileMode)
	if err != nil {
		return nil, err
	}
	if err := opened.Chmod(fileMode); err != nil {
		_ = opened.Close()
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
	file, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fileMode)
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
// package stamps it keeps no time. It reads the end of the file first, the
// whole of it only when that end holds fewer than n lines.
func Last(file run.LogFile, n int) []run.Line {
	path := Path(file.Dir, file.Job)
	if path == "" || n <= 0 {
		return nil
	}
	lines, whole := tail(path)
	if len(lines) < n && !whole {
		lines, whole = parsed(path, 0)
	}
	if len(lines) < n && whole {
		older, _ := parsed(backup(path, 1), 0)
		lines = append(older, lines...)
	}

	return lines[max(0, len(lines)-n):]
}

// tail returns the lines of the last tailRead bytes of the file at path, and
// whether they are the whole file.
func tail(path string) ([]run.Line, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, true
	}
	return parsed(path, max(0, info.Size()-tailRead))
}

// parsed returns the lines of the file at path from offset on, the line cut
// at offset left out, and whether they are the whole file.
func parsed(path string, offset int64) ([]run.Line, bool) {
	file, err := os.Open(path)
	if err != nil {
		return nil, true
	}
	defer file.Close()
	data, err := io.ReadAll(io.NewSectionReader(file, offset, math.MaxInt64-offset))
	if err != nil {
		return nil, true
	}
	text := string(data)
	if offset > 0 {
		if _, rest, found := strings.Cut(text, "\n"); found {
			text = rest
		}
	}
	var lines []run.Line
	for line := range strings.Lines(text) {
		lines = append(lines, parse(strings.TrimSuffix(line, "\n")))
	}

	return lines, offset == 0
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
