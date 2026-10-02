package execsvc

import (
	"strings"
	"testing"
)

func TestTailKeepsTheLastLines(t *testing.T) {
	tail := newTail(3)
	_, _ = tail.Write([]byte("1\n2\n3\n4"))
	_, _ = tail.Write([]byte("\n5\n"))
	if got := strings.Join(tail.Lines(), ","); got != "3,4,5" {
		t.Fatalf("got %q", got)
	}
}

func TestTailFlushesAnUnterminatedLastLine(t *testing.T) {
	tail := newTail(3)
	_, _ = tail.Write([]byte("a\nno newline"))
	if got := strings.Join(tail.Lines(), ","); got != "a,no newline" {
		t.Fatalf("got %q", got)
	}
}

func TestTailCapsALineThatNeverEnds(t *testing.T) {
	tail := newTail(3)
	chunk := []byte(strings.Repeat("x", 1024))
	for range 1000 {
		_, _ = tail.Write(chunk)
	}
	lines := tail.Lines()
	if len(lines) != 1 || len(lines[0]) > 4096 {
		t.Fatalf("partial line not capped: %d lines, %d bytes", len(lines), len(lines[0]))
	}
}

func TestTailKeepsWhatATerminalWouldShow(t *testing.T) {
	tail := newTail(5)
	_, _ = tail.Write([]byte("progress 10%\rprogress 100%\rdone\n\x1b[31mred\x1b[0m\nstatus\r\n"))
	if got := strings.Join(tail.Lines(), ","); got != "done,red,status" {
		t.Fatalf("got %q", got)
	}
}
