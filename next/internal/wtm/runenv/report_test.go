package runenv

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/wtm/dotenv"
	"github.com/LucasPcq/wtm/next/internal/wtm/run"
	"github.com/stretchr/testify/assert"
)

func TestTheAnomalyLinesNameEachRefusalGroupedByFile(t *testing.T) {
	plan := planPorts(planPortsParams{
		Links: []run.EnvPortLink{
			{File: ".env", Key: "MISSING", Job: "svc", Port: "DB_PORT"},
			{File: ".env", Key: "TWICE", Job: "svc", Port: "API_PORT"},
			{File: ".env", Key: "ELSEWHERE", Job: "svc", Port: "DB_PORT"},
		},
		Bases:  map[portRef]int{{Job: "svc", Name: "DB_PORT"}: 5432, {Job: "svc", Name: "API_PORT"}: 3000},
		Offset: 10,
		Lines: map[string][]dotenv.Line{
			".env": dotenv.Parse([]byte("TWICE=http://a:3000/b:3000\nELSEWHERE=postgres://localhost:6000/app\n")),
		},
	})

	joined := strings.Join(AnomalyLines(Redact(plan)), "\n")

	for _, key := range []string{"MISSING", "TWICE", "ELSEWHERE"} {
		assert.Contains(t, joined, key)
	}
	assert.Contains(t, joined, "── .env ")
	assert.NotContains(t, joined, "port DB_PORT", "the port is its own column, no reason repeats it")
}

func TestThePlanWithNoRefusalHasNoAnomalyLine(t *testing.T) {
	assert.Empty(t, AnomalyLines(Redacted{Entries: []Entry{{File: ".env", Key: "PORT", Status: StatusRewrite}}}))
}

func TestTheSettlementNoteCountsPortsAndOwnedValues(t *testing.T) {
	rewrite := Entry{Status: StatusRewrite}
	unchanged := Entry{Status: StatusUnchanged}
	written := OwnedEntry{Key: "REALM", Changed: true}
	kept := OwnedEntry{Key: "COMPOSE_PROJECT_NAME"}
	cases := []struct {
		name string
		plan Redacted
		want string
	}{
		{name: "nothing moved", plan: Redacted{Offset: 20, Entries: []Entry{unchanged}, Owned: []OwnedEntry{kept}}, want: ""},
		{name: "one port", plan: Redacted{Offset: 20, Entries: []Entry{rewrite, unchanged}}, want: "1 port settled (offset +20)"},
		{name: "several ports", plan: Redacted{Offset: 10, Entries: []Entry{rewrite, rewrite}}, want: "2 ports settled (offset +10)"},
		{name: "owned values only", plan: Redacted{Owned: []OwnedEntry{written, kept, written}}, want: "2 owned values written"},
		{name: "both", plan: Redacted{Offset: 10, Entries: []Entry{rewrite}, Owned: []OwnedEntry{written}}, want: "1 port settled (offset +10) · 1 owned value written"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, SettlementNote(tc.plan))
		})
	}
}
