package rules_test

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func TestRunsOnPipe(t *testing.T) {
	if !rules.RunsOnPipe(domain.JobKindTask) {
		t.Error("une task sort sur un pipe, sans discipline de ligne")
	}
	if rules.RunsOnPipe(domain.JobKindService) {
		t.Error("un service sort sur un PTY, qui termine ses lignes lui-même")
	}
}

func TestNormalizeEOL(t *testing.T) {
	for _, tc := range []struct {
		name      string
		chunk     string
		pendingCR bool
		want      string
		wantCR    bool
	}{
		{name: "bare LF gains a CR", chunk: "a\nb\n", want: "a\r\nb\r\n"},
		{name: "CRLF is left alone", chunk: "a\r\nb\r\n", want: "a\r\nb\r\n"},
		{name: "lone CR is left alone", chunk: "50%\r75%\r", want: "50%\r75%\r", wantCR: true},
		{name: "no line break at all", chunk: "spinner", want: "spinner"},
		{name: "empty chunk keeps the carry", chunk: "", pendingCR: true, want: "", wantCR: true},
		// Le CRLF coupé entre deux chunks est le seul cas qu'un remplacement
		// naïf casse : le LF ouvrant le chunk suivant termine un CRLF déjà écrit.
		{name: "LF completing a split CRLF", chunk: "\nnext", pendingCR: true, want: "\nnext"},
		{name: "LF opening a chunk after a plain byte", chunk: "\nnext", want: "\r\nnext"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := rules.NormalizeEOL(rules.NormalizeEOLParams{
				Chunk:     []byte(tc.chunk),
				PendingCR: tc.pendingCR,
			})
			if string(got.Chunk) != tc.want {
				t.Errorf("chunk = %q, want %q", got.Chunk, tc.want)
			}
			if got.PendingCR != tc.wantCR {
				t.Errorf("pendingCR = %v, want %v", got.PendingCR, tc.wantCR)
			}
		})
	}
}

// Deux chunks consécutifs doivent se recoller sans CR en double, ce que seul le
// report de PendingCR garantit.
func TestNormalizeEOLAcrossChunks(t *testing.T) {
	first := rules.NormalizeEOL(rules.NormalizeEOLParams{Chunk: []byte("seeding\r")})
	second := rules.NormalizeEOL(rules.NormalizeEOLParams{Chunk: []byte("\ndone\n"), PendingCR: first.PendingCR})

	if joined := string(first.Chunk) + string(second.Chunk); joined != "seeding\r\ndone\r\n" {
		t.Errorf("joined = %q, want %q", joined, "seeding\r\ndone\r\n")
	}
}
