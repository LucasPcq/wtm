package rules

import (
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

type WorktreeLogDirParams struct {
	StateDir string
	Branch   string
}

// WorktreeLogDir returns <state-dir>/logs/<encoded-branch>/. Resolved
// client-side and handed to the daemon, which must never resolve a git dir of
// its own. The branch keys the directory the same way <state-dir>/worktrees/
// does — the state dir is shared by every worktree of the repository, so two
// worktrees whose paths share a base name would otherwise interleave their
// writes and purge each other. A branch that would not survive as a single path
// segment resolves to nothing: this path feeds a recursive delete, so refusing
// beats guessing.
func WorktreeLogDir(params WorktreeLogDirParams) string {
	if params.StateDir == "" || params.Branch == "" {
		return ""
	}
	segment := EncodeBranchSegment(params.Branch)
	if !isSinglePathSegment(segment) {
		return ""
	}
	return filepath.Join(params.StateDir, domain.JobLogsDirName, segment)
}

func isSinglePathSegment(segment string) bool {
	if segment == "" || segment == "." || segment == ".." {
		return false
	}
	return !strings.ContainsAny(segment, `/\`)
}

// JobLogFileName names a job's log file, percent-escaping rather than filtering
// the job name — the same encoding the directory above uses. Mapping every
// unusual rune onto '_' made `web api` and `web:api` share one file, which
// merely interleaved their lines until a start began emptying it (LUC-198).
// Escaping also keeps a readable name: only a rune that cannot sit in a path
// segment grows a %XX.
func JobLogFileName(job string) string {
	escaped := url.PathEscape(job)
	if escaped == "" {
		return ""
	}
	return escaped + domain.JobLogFileExt
}

// terminalEscape matches what a PTY-backed job writes to drive the terminal:
// CSI (colors, cursor moves, line clears), OSC (window title), charset
// designation, and the single-byte escapes.
var terminalEscape = regexp.MustCompile(`\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)?|\x1b\[[0-9;?:<=>]*[ -/]*[@-~]|\x1b[()*+#][0-9A-Za-z]|\x1b[@-Z\\-_]`)

func StripTerminalEscapes(s string) string {
	return terminalEscape.ReplaceAllString(s, "")
}

type SanitizeChunkParams struct {
	Chunk   string
	Pending string
	At      time.Time
}

type SanitizeChunkResult struct {
	Records []domain.LogRecord
	// Pending is the tail still waiting for its newline — feed it back as the
	// next call's Pending.
	Pending string
}

// SanitizeLogChunk turns raw output into whole plain-text lines. Escape
// sequences are left in the carried-over tail, which is what makes a chunk
// boundary falling in the middle of one — or of a line — harmless. The tail is
// bounded on both counts a job can breach it: redraw frames are collapsed as
// they arrive, and a line that never ends is journaled once it grows past
// domain.JobLogMaxPendingBytes.
func SanitizeLogChunk(params SanitizeChunkParams) SanitizeChunkResult {
	segments := strings.Split(params.Pending+params.Chunk, "\n")

	result := SanitizeChunkResult{}
	for _, segment := range segments[:len(segments)-1] {
		result.Records = appendRecord(result.Records, params.At, SanitizeLogLine(segment))
	}

	pending := collapseRedraws(segments[len(segments)-1])
	if len(pending) <= domain.JobLogMaxPendingBytes {
		result.Pending = pending
		return result
	}
	result.Records = appendRecord(result.Records, params.At, SanitizeLogLine(pending))
	return result
}

func SanitizeLogLine(raw string) string {
	return strings.TrimRight(stripControlRunes(StripTerminalEscapes(collapseRedraws(raw))), " \t")
}

// collapseRedraws keeps only the last state of a line written over itself with
// carriage returns: a progress bar leaves its final value in the log, not each
// of its frames. A trailing \r is kept — it may be the CR of a CRLF whose LF is
// still in the next chunk.
func collapseRedraws(raw string) string {
	end := len(raw)
	if strings.HasSuffix(raw, "\r") {
		end--
	}
	last := strings.LastIndex(raw[:end], "\r")
	if last < 0 {
		return raw
	}
	return raw[last+1:]
}

func appendRecord(records []domain.LogRecord, at time.Time, text string) []domain.LogRecord {
	if text == "" {
		return records
	}
	return append(records, domain.LogRecord{At: at, Text: text})
}

func FormatLogRecord(record domain.LogRecord) string {
	return record.At.UTC().Format(domain.JobLogTimestampLayout) + domain.JobLogSeparator + record.Text
}

type ParseLogLineParams struct {
	Job  string
	Line string
}

// ParseLogLine reads back what FormatLogRecord wrote. A line it cannot date is
// carried whole as text rather than dropped: a log file predating this format,
// or a line a sink wrote unstamped, still says something.
func ParseLogLine(params ParseLogLineParams) domain.JobLogEntry {
	stamp, text, found := strings.Cut(params.Line, domain.JobLogSeparator)
	if !found {
		return domain.JobLogEntry{Job: params.Job, Text: params.Line}
	}
	if _, err := time.Parse(domain.JobLogTimestampLayout, stamp); err != nil {
		return domain.JobLogEntry{Job: params.Job, Text: params.Line}
	}
	return domain.JobLogEntry{Job: params.Job, At: stamp, Text: text}
}

func stripControlRunes(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\t' {
			return r
		}
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
}

// JobFromLogFileName is JobLogFileName read backwards, rotation suffix
// included: a job whose only surviving output is in `web.log.2` has still left
// a trace. It answers empty for anything that is not a job log, so a directory
// holding something else never invents a job.
func JobFromLogFileName(name string) string {
	trimmed := name
	// Rotation appends `.N` after the extension, so the rank comes off first.
	if ext := filepath.Ext(trimmed); ext != "" && ext != domain.JobLogFileExt {
		if _, err := strconv.Atoi(strings.TrimPrefix(ext, ".")); err != nil {
			return ""
		}
		trimmed = strings.TrimSuffix(trimmed, ext)
	}
	if !strings.HasSuffix(trimmed, domain.JobLogFileExt) {
		return ""
	}
	job, err := url.PathUnescape(strings.TrimSuffix(trimmed, domain.JobLogFileExt))
	if err != nil {
		return ""
	}
	return job
}
