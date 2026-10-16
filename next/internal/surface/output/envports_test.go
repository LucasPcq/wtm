package output

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/wtm/settle"
	"github.com/stretchr/testify/assert"
)

func TestTheAnomalyLinesNameEachRefusalGroupedByFile(t *testing.T) {
	plan := settle.Redacted{Entries: []settle.Entry{
		{File: ".env", Key: "MISSING", Port: "DB_PORT", Base: 5432, Status: settle.StatusMissingKey},
		{File: ".env", Key: "TWICE", Port: "API_PORT", Base: 3000, Status: settle.StatusAmbiguous},
		{File: "api/.env", Key: "ELSEWHERE", Port: "DB_PORT", Base: 5432, Status: settle.StatusNotFound},
		{File: ".env", Key: "MOVED", Port: "DB_PORT", Status: settle.StatusRewrite},
	}}

	assert.Equal(t, []string{
		"── .env ──────────────────────────────────────────",
		"MISSING    DB_PORT   no such key in this file",
		"TWICE      API_PORT  3000 appears more than once",
		"",
		"── api/.env ──────────────────────────────────────",
		"ELSEWHERE  DB_PORT   no 5432 to shift in the value",
	}, AnomalyLines(plan))
}

func TestAnAnomalyLineNeverNamesAForeignHost(t *testing.T) {
	plan := settle.Redact(settle.Plan{Entries: []settle.Entry{{File: ".env", Key: "API", Port: "P", Status: settle.StatusForeignHost, ForeignHost: "u:pw@api.staging.example.com"}}})

	joined := strings.Join(AnomalyLines(plan), "\n")

	assert.Contains(t, joined, "points at another host, which no job here serves")
	assert.NotContains(t, joined, "staging")
}

func TestAPlanWithNoRefusalHasNoAnomalyLine(t *testing.T) {
	assert.Empty(t, AnomalyLines(settle.Redacted{Entries: []settle.Entry{{File: ".env", Key: "PORT", Status: settle.StatusRewrite}}}))
}

func TestTheSettlementNoteCountsPortsAndOwnedValues(t *testing.T) {
	rewrite := settle.Entry{Status: settle.StatusRewrite}
	unchanged := settle.Entry{Status: settle.StatusUnchanged}
	written := settle.OwnedEntry{Key: "REALM", Changed: true}
	kept := settle.OwnedEntry{Key: "COMPOSE_PROJECT_NAME"}
	cases := []struct {
		name string
		plan settle.Redacted
		want string
	}{
		{name: "nothing moved", plan: settle.Redacted{Offset: 20, Entries: []settle.Entry{unchanged}, Owned: []settle.OwnedEntry{kept}}, want: ""},
		{name: "one port", plan: settle.Redacted{Offset: 20, Entries: []settle.Entry{rewrite, unchanged}}, want: "1 port settled (offset +20)"},
		{name: "several ports", plan: settle.Redacted{Offset: 10, Entries: []settle.Entry{rewrite, rewrite}}, want: "2 ports settled (offset +10)"},
		{name: "owned values only", plan: settle.Redacted{Owned: []settle.OwnedEntry{written, kept, written}}, want: "2 owned values written"},
		{name: "both", plan: settle.Redacted{Offset: 10, Entries: []settle.Entry{rewrite}, Owned: []settle.OwnedEntry{written}}, want: "1 port settled (offset +10) · 1 owned value written"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, SettlementNote(tc.plan))
		})
	}
}
