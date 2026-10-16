package shell

import (
	"bytes"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	hookLogDir       = "hooks"
	hookLogSeparator = "-"
	hookLogExt       = ".log"
)

type HookLogParams struct {
	StateDir string
	Phase    string
	Branch   string
}

func HookLogPath(params HookLogParams) string {
	if params.StateDir == "" {
		return ""
	}
	name := params.Phase
	if params.Branch != "" {
		name += hookLogSeparator + url.PathEscape(params.Branch)
	}
	return filepath.Join(params.StateDir, hookLogDir, name+hookLogExt)
}

type linesParams struct {
	Emit func(string)
	Log  string
}

type lines struct {
	mu      sync.Mutex
	emit    func(string)
	logPath string
	opened  bool
	log     io.WriteCloser
	buf     bytes.Buffer
}

func newLines(params linesParams) *lines {
	return &lines{emit: params.Emit, logPath: params.Log}
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

func (l *lines) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.buf.Write(p)
	for {
		line, err := l.buf.ReadString('\n')
		if err != nil {
			l.buf.Reset()
			l.buf.WriteString(line)
			return len(p), nil
		}
		l.send(strings.TrimSuffix(line, "\n"))
	}
}

func (l *lines) flush() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.buf.Len() == 0 {
		return
	}
	rest := l.buf.String()
	l.buf.Reset()
	l.send(rest)
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
