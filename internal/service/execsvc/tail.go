// Package execsvc runs one shell line in several worktrees at once.
package execsvc

import (
	"bytes"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

type tail struct {
	max     int
	lines   []string
	partial []byte
}

func newTail(max int) *tail { return &tail{max: max} }

func (t *tail) Write(p []byte) (int, error) {
	data := append(t.partial, p...)
	for {
		i := bytes.IndexByte(data, '\n')
		if i < 0 {
			break
		}
		t.push(string(bytes.TrimRight(data[:i], "\r")))
		data = data[i+1:]
	}
	if len(data) > domain.ExecPartialLineCap {
		data = data[len(data)-domain.ExecPartialLineCap:]
	}
	t.partial = append([]byte(nil), data...)
	return len(p), nil
}

func (t *tail) push(line string) {
	t.lines = append(t.lines, rules.TerminalLine(line))
	if len(t.lines) > t.max {
		t.lines = t.lines[len(t.lines)-t.max:]
	}
}

func (t *tail) Lines() []string {
	if len(t.partial) == 0 {
		return t.lines
	}
	lines := append(append([]string(nil), t.lines...), rules.TerminalLine(string(t.partial)))
	if len(lines) > t.max {
		lines = lines[len(lines)-t.max:]
	}
	return lines
}
