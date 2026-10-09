package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestDaemonStatusReportNamesAReadOnlyIndex(t *testing.T) {
	var buf bytes.Buffer
	DaemonStatusReport(&buf, domain.DaemonStatus{
		Running:     true,
		Version:     "1.2.3",
		StatePath:   "/home/x/.config/wtm/jobs.json",
		IndexFrozen: true,
	})

	out := buf.String()
	if !strings.Contains(out, domain.DaemonIndexFrozenTitle) {
		t.Fatalf("a read-only index must be named, not left silent:\n%s", out)
	}
	if !strings.Contains(out, "/home/x/.config/wtm/jobs.json") {
		t.Errorf("the callout must name the file the reader has to act on:\n%s", out)
	}
}

func TestDaemonStatusReportStaysQuietOnAWritableIndex(t *testing.T) {
	var buf bytes.Buffer
	DaemonStatusReport(&buf, domain.DaemonStatus{Running: true, Version: "1.2.3"})

	if strings.Contains(buf.String(), domain.DaemonIndexFrozenTitle) {
		t.Fatal("the nominal path must say nothing about the index")
	}
}
