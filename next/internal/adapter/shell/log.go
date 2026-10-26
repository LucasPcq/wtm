package shell

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// lines is the sink of a hook chain's output: it splits what the hooks write
// on stdout and stderr into lines and hands each to emit and to the log file, opened
// on the first hook that runs. Each stream keeps its own unfinished line, so a
// line half written on stdout never merges with one written on stderr.
type lines struct {
	mu      sync.Mutex
	emit    func(string)
	logPath string
	opened  bool
	log     io.WriteCloser
	stdout  stream
	stderr  stream
}

// A stream is one output of a hook, stdout or stderr, writing into its sink.
type stream struct {
	sink *lines
	buf  bytes.Buffer
}

func newLines(emit func(string), log string) *lines {
	l := &lines{emit: emit, logPath: log}
	l.stdout.sink, l.stderr.sink = l, l
	return l
}

func (l *lines) openLog() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.opened {
		return
	}

	l.opened = true
	l.log = openLog(l.logPath)
}

// openLog creates the log file at path, its directory included, and returns
// nil when path is empty or the file cannot be created: a log never fails a
// hook.
func openLog(path string) io.WriteCloser {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil
	}

	file, err := os.Create(path)
	if err != nil {
		return nil
	}
	return file
}

// Write sends every complete line of p, the rest of an earlier write before
// it, and keeps an unfinished last line for the next write or flush.
func (s *stream) Write(p []byte) (int, error) {
	s.sink.mu.Lock()
	defer s.sink.mu.Unlock()
	s.buf.Write(p)

	for {
		line, err := s.buf.ReadString('\n')
		if err != nil {
			s.buf.Reset()
			s.buf.WriteString(line)
			return len(p), nil
		}
		s.sink.send(strings.TrimSuffix(line, "\n"))
	}
}

// flush sends the unfinished last line a hook left on each stream, if any.
func (l *lines) flush() {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, s := range []*stream{&l.stdout, &l.stderr} {
		if s.buf.Len() == 0 {
			continue
		}
		rest := s.buf.String()
		s.buf.Reset()
		l.send(rest)
	}
}

func (l *lines) send(line string) {
	if l.log != nil {
		_, _ = io.WriteString(l.log, line+"\n")
	}
	if l.emit != nil {
		l.emit(line)
	}
}

func (l *lines) close() {
	if l.log != nil {
		_ = l.log.Close()
	}
}
