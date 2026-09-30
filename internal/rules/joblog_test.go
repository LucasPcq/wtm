package rules

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestStripTerminalEscapes(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"colors", "\x1b[32mready\x1b[0m", "ready"},
		{"line erase", "building\x1b[2K\x1b[1G", "building"},
		{"cursor move", "\x1b[1;1Hhome", "home"},
		{"private mode", "\x1b[?25lhidden\x1b[?25h", "hidden"},
		{"window title", "\x1b]0;vite dev\x07serving", "serving"},
		{"charset designation", "\x1b(Bplain", "plain"},
		{"single byte escape", "wrapped\x1bMnext", "wrappednext"},
		{"nothing to strip", "plain text", "plain text"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := StripTerminalEscapes(tc.raw); got != tc.want {
				t.Errorf("StripTerminalEscapes(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestSanitizeLogLine(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"crlf", "done\r", "done"},
		{"progress redraw keeps the last frame", "[+] Running 1/3\r[+] Running 2/3\r[+] Running 3/3", "[+] Running 3/3"},
		{"colored redraw", "\x1b[33mbuild 10%\x1b[0m\r\x1b[32mbuild 100%\x1b[0m", "build 100%"},
		{"trailing spaces from a padded redraw", "ready       ", "ready"},
		{"stray control bytes", "bell\x07here", "bellhere"},
		{"tabs survive", "col1\tcol2", "col1\tcol2"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := SanitizeLogLine(tc.raw); got != tc.want {
				t.Errorf("SanitizeLogLine(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func texts(records []domain.LogRecord) []string {
	out := make([]string, 0, len(records))
	for _, record := range records {
		out = append(out, record.Text)
	}
	return out
}

func TestSanitizeLogChunkSplitsCompleteLinesOnly(t *testing.T) {
	at := time.Date(2026, 8, 20, 12, 4, 11, 0, time.UTC)
	result := SanitizeLogChunk(SanitizeChunkParams{Chunk: "first\nsecond\nthir", At: at})

	if got := texts(result.Records); strings.Join(got, "|") != "first|second" {
		t.Errorf("records = %v, want the two complete lines", got)
	}
	if result.Pending != "thir" {
		t.Errorf("pending = %q, want the incomplete tail", result.Pending)
	}
	for _, record := range result.Records {
		if !record.At.Equal(at) {
			t.Errorf("record stamped %v, want %v", record.At, at)
		}
	}
}

func TestSanitizeLogChunkJoinsALineSplitAcrossChunks(t *testing.T) {
	first := SanitizeLogChunk(SanitizeChunkParams{Chunk: "half of a "})
	if len(first.Records) != 0 {
		t.Fatalf("records = %v, want nothing before the newline", texts(first.Records))
	}

	second := SanitizeLogChunk(SanitizeChunkParams{Chunk: "line\n", Pending: first.Pending})
	if got := texts(second.Records); len(got) != 1 || got[0] != "half of a line" {
		t.Errorf("records = %v, want the rejoined line", got)
	}
	if second.Pending != "" {
		t.Errorf("pending = %q, want it drained", second.Pending)
	}
}

func TestSanitizeLogChunkJoinsAnEscapeSplitAcrossChunks(t *testing.T) {
	first := SanitizeLogChunk(SanitizeChunkParams{Chunk: "hello \x1b[3"})
	second := SanitizeLogChunk(SanitizeChunkParams{Chunk: "2mworld\x1b[0m\n", Pending: first.Pending})

	if got := texts(second.Records); len(got) != 1 || got[0] != "hello world" {
		t.Errorf("records = %v, want the escape stripped across the boundary", got)
	}
}

func TestSanitizeLogChunkHandlesCRLFAndProgressBars(t *testing.T) {
	raw := "\x1b[1mturbo\x1b[0m\r\n" +
		"downloading 10%\rdownloading 60%\rdownloading 100%\r\n" +
		"\r\n" +
		"done\n"

	result := SanitizeLogChunk(SanitizeChunkParams{Chunk: raw})

	want := []string{"turbo", "downloading 100%", "done"}
	got := texts(result.Records)
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("records = %v, want %v", got, want)
	}
}

func TestFormatLogRecord(t *testing.T) {
	record := domain.LogRecord{
		At:   time.Date(2026, 8, 20, 12, 4, 11, 0, time.FixedZone("CEST", 2*60*60)),
		Text: "listening on :3000",
	}

	if got := FormatLogRecord(record); got != "2026-08-20T10:04:11Z  listening on :3000" {
		t.Errorf("FormatLogRecord = %q, want a UTC-stamped line", got)
	}
}

func TestWorktreeLogDir(t *testing.T) {
	got := WorktreeLogDir(WorktreeLogDirParams{StateDir: "/repo/.git/wtm", Branch: "feat/login"})
	if got != "/repo/.git/wtm/logs/feat%2Flogin" {
		t.Errorf("WorktreeLogDir = %q, want the encoded branch under logs/", got)
	}

	if got := WorktreeLogDir(WorktreeLogDirParams{Branch: "feat"}); got != "" {
		t.Errorf("WorktreeLogDir without a state dir = %q, want nothing to persist", got)
	}
	if got := WorktreeLogDir(WorktreeLogDirParams{StateDir: "/repo/.git/wtm"}); got != "" {
		t.Errorf("WorktreeLogDir without a branch = %q, want nothing to persist", got)
	}
}

func TestWorktreeLogDirKeepsSameNamedWorktreesApart(t *testing.T) {
	first := WorktreeLogDir(WorktreeLogDirParams{StateDir: "/repo/.git/wtm", Branch: "alice/api"})
	second := WorktreeLogDir(WorktreeLogDirParams{StateDir: "/repo/.git/wtm", Branch: "bob/api"})

	if first == second {
		t.Errorf("two worktrees checked out at .../api share %q — the state dir is shared by the whole repo", first)
	}
}

func TestWorktreeLogDirRefusesABranchThatEscapesItsSegment(t *testing.T) {
	for _, branch := range []string{".", ".."} {
		if got := WorktreeLogDir(WorktreeLogDirParams{StateDir: "/repo/.git/wtm", Branch: branch}); got != "" {
			t.Errorf("WorktreeLogDir(%q) = %q, want nothing rather than a path a purge would delete", branch, got)
		}
	}

	// Anything else stays inside logs/ because the encoding leaves no separator.
	for _, branch := range []string{"/", "../../etc", "a/../..", "\\", "%2F.."} {
		got := WorktreeLogDir(WorktreeLogDirParams{StateDir: "/repo/.git/wtm", Branch: branch})
		if filepath.Dir(got) != "/repo/.git/wtm/logs" {
			t.Errorf("WorktreeLogDir(%q) = %q, want a single segment under logs/", branch, got)
		}
	}
}

func TestJobLogFileNameEscapesPathSeparators(t *testing.T) {
	cases := map[string]string{
		"web":            "web.log",
		"web-api":        "web-api.log",
		"api-db:migrate": "api-db:migrate.log",
		"api/gateway":    "api%2Fgateway.log",
		"../escape":      "..%2Fescape.log",
		"db 2":           "db%202.log",
		"":               "",
	}
	for job, want := range cases {
		if got := JobLogFileName(job); got != want {
			t.Errorf("JobLogFileName(%q) = %q, want %q", job, got, want)
		}
	}
}

// Two jobs never share a file. Folding every unusual rune onto '_' made
// distinct names collide, which merely interleaved their lines until a start
// began emptying the file it opens (LUC-198).
func TestJobLogFileNameNeverCollides(t *testing.T) {
	names := []string{"web api", "web:api", "web_api", "web/api", "web%20api", "web.api", "web-api"}

	seen := map[string]string{}
	for _, name := range names {
		file := JobLogFileName(name)
		if strings.ContainsAny(file, `/\`) {
			t.Errorf("JobLogFileName(%q) = %q, which is not a single path segment", name, file)
		}
		if other, taken := seen[file]; taken {
			t.Errorf("%q and %q both map to %q", other, name, file)
		}
		seen[file] = name
	}
}

func TestSanitizeLogChunkCollapsesRedrawsIntoThePendingTail(t *testing.T) {
	pending := ""
	for i := 0; i < 500; i++ {
		result := SanitizeLogChunk(SanitizeChunkParams{
			Chunk:   "\r\x1b[K[" + strings.Repeat("=", i%20) + "] building",
			Pending: pending,
		})
		if len(result.Records) != 0 {
			t.Fatalf("frame %d journaled %v, want a redraw to stay pending", i, texts(result.Records))
		}
		pending = result.Pending
	}

	if len(pending) > 64 {
		t.Errorf("pending holds %d bytes after 500 redraws, want only the last frame", len(pending))
	}
	if got := SanitizeLogLine(pending); got != "[===================] building" {
		t.Errorf("the pending tail reads %q, want the last frame", got)
	}
}

func TestSanitizeLogChunkFlushesATailThatNeverEnds(t *testing.T) {
	result := SanitizeLogChunk(SanitizeChunkParams{Chunk: strings.Repeat("x", domain.JobLogMaxPendingBytes+1)})

	if len(result.Records) != 1 {
		t.Fatalf("records = %d, want the oversized tail journaled", len(result.Records))
	}
	if result.Pending != "" {
		t.Errorf("pending holds %d bytes, want it flushed", len(result.Pending))
	}
	if len(result.Records[0].Text) != domain.JobLogMaxPendingBytes+1 {
		t.Errorf("flushed record is %d bytes, want the whole tail", len(result.Records[0].Text))
	}
}

func TestSanitizeLogChunkKeepsACRLFSplitAcrossChunks(t *testing.T) {
	first := SanitizeLogChunk(SanitizeChunkParams{Chunk: "done\r"})
	if len(first.Records) != 0 {
		t.Fatalf("records = %v, want nothing before the newline", texts(first.Records))
	}

	second := SanitizeLogChunk(SanitizeChunkParams{Chunk: "\nnext", Pending: first.Pending})
	if got := texts(second.Records); len(got) != 1 || got[0] != "done" {
		t.Errorf("records = %v, want the line whose CR and LF straddled the boundary", got)
	}
}

func TestSanitizeLogChunkOnBinaryOutput(t *testing.T) {
	raw := "before\x00\xff\xfe\x01after\n\x1b[31m\xc3\x28 broken utf8\x1b[0m\n"

	result := SanitizeLogChunk(SanitizeChunkParams{Chunk: raw})

	got := texts(result.Records)
	if len(got) != 2 {
		t.Fatalf("records = %v, want two lines", got)
	}
	if got[0] != "before\ufffd\ufffdafter" {
		t.Errorf("first line = %q, want the control bytes dropped and the invalid ones replaced", got[0])
	}
	if got[1] != "\ufffd( broken utf8" {
		t.Errorf("second line = %q, want the escapes stripped and the text kept", got[1])
	}
	for _, record := range result.Records {
		if strings.ContainsAny(record.Text, "\x00\x01\x1b\n\r") {
			t.Errorf("record %q still carries control bytes", record.Text)
		}
		if !utf8.ValidString(record.Text) {
			t.Errorf("record %q is not valid UTF-8", record.Text)
		}
	}
}

func TestParseLogLineReadsBackWhatFormatWrote(t *testing.T) {
	at := time.Date(2026, 9, 2, 10, 4, 11, 0, time.UTC)
	line := FormatLogRecord(domain.LogRecord{At: at, Text: "listening on 3000"})

	got := ParseLogLine(ParseLogLineParams{Job: "api", Line: line})
	want := domain.JobLogEntry{Job: "api", At: "2026-09-02T10:04:11Z", Text: "listening on 3000"}
	if got != want {
		t.Errorf("ParseLogLine = %+v, want %+v", got, want)
	}
}

// A line whose text carries the separator keeps all of it: only the first cut
// is the stamp.
func TestParseLogLineKeepsASeparatorInsideTheText(t *testing.T) {
	got := ParseLogLine(ParseLogLineParams{Job: "api", Line: "2026-09-02T10:04:11Z  GET /  200"})
	if got.Text != "GET /  200" {
		t.Errorf("text = %q, want the whole line after the stamp", got.Text)
	}
}

// A log written before this format is still worth handing over: it loses its
// date, not its content.
func TestParseLogLineCarriesAnUndatableLineWhole(t *testing.T) {
	for _, line := range []string{"listening on 3000", "not a date  listening on 3000"} {
		got := ParseLogLine(ParseLogLineParams{Job: "api", Line: line})
		if got.At != "" || got.Text != line {
			t.Errorf("ParseLogLine(%q) = %+v, want the line whole and undated", line, got)
		}
	}
}
