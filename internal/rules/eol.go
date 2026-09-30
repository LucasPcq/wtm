package rules

import (
	"bytes"

	"github.com/LucasPcq/wtm/internal/domain"
)

// RunsOnPipe reports that a job's output reaches its reader through a plain
// pipe rather than a PTY. Only a PTY carries a line discipline, so only a PTY's
// output is already terminated the way a terminal emulator expects.
func RunsOnPipe(kind domain.JobKind) bool {
	return kind == domain.JobKindTask
}

type NormalizeEOLParams struct {
	Chunk []byte
	// PendingCR says the previous chunk ended on a CR. The LF opening this one
	// then completes a CRLF the job already wrote, and must not gain a second.
	PendingCR bool
}

type NormalizeEOLResult struct {
	Chunk     []byte
	PendingCR bool
}

// NormalizeEOL turns the bare LFs of a job running on a pipe into the CRLFs a
// terminal emulator needs. A PTY does this itself (ONLCR); a pipe has no line
// discipline, so an emulator reading its output moves down a row without
// returning to column zero and every line lands one step further right.
func NormalizeEOL(params NormalizeEOLParams) NormalizeEOLResult {
	if !bytes.ContainsRune(params.Chunk, '\n') {
		return NormalizeEOLResult{
			Chunk:     params.Chunk,
			PendingCR: endsOnCR(params.Chunk, params.PendingCR),
		}
	}

	out := make([]byte, 0, len(params.Chunk)+bytes.Count(params.Chunk, []byte{'\n'}))
	prevCR := params.PendingCR
	for _, b := range params.Chunk {
		if b == '\n' && !prevCR {
			out = append(out, '\r')
		}
		out = append(out, b)
		prevCR = b == '\r'
	}
	return NormalizeEOLResult{Chunk: out, PendingCR: prevCR}
}

func endsOnCR(chunk []byte, fallback bool) bool {
	if len(chunk) == 0 {
		return fallback
	}
	return chunk[len(chunk)-1] == '\r'
}
