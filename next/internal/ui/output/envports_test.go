package output

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/env"

	"github.com/stretchr/testify/assert"
)

func TestTheAnomalyLinesNameEachRefusalGroupedByFile(t *testing.T) {
	plan := env.Redacted{Entries: []env.Entry{
		{File: ".env", Key: "MISSING", Port: "DB_PORT", Base: 5432, Status: env.StatusMissingKey},
		{File: ".env", Key: "TWICE", Port: "API_PORT", Base: 3000, Status: env.StatusAmbiguous},
		{File: "api/.env", Key: "ELSEWHERE", Port: "DB_PORT", Base: 5432, Status: env.StatusNotFound},
		{File: ".env", Key: "MOVED", Port: "DB_PORT", Status: env.StatusRewrite},
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
	plan := env.Redact(env.Plan{Entries: []env.Entry{{File: ".env", Key: "API", Port: "P", Status: env.StatusForeignHost, ForeignHost: "u:pw@api.staging.example.com"}}})

	joined := strings.Join(AnomalyLines(plan), "\n")

	assert.Contains(t, joined, "points at another host, which no job here serves")
	assert.NotContains(t, joined, "staging")
}

func TestAPlanWithNoRefusalHasNoAnomalyLine(t *testing.T) {
	assert.Empty(t, AnomalyLines(env.Redacted{Entries: []env.Entry{{File: ".env", Key: "PORT", Status: env.StatusRewrite}}}))
}

func TestTheSettlementNoteCountsPortsAndOwnedValues(t *testing.T) {
	rewrite := env.Entry{Status: env.StatusRewrite}
	unchanged := env.Entry{Status: env.StatusUnchanged}
	written := env.OwnedEntry{Key: "REALM", Changed: true}
	kept := env.OwnedEntry{Key: "COMPOSE_PROJECT_NAME"}
	cases := []struct {
		name string
		plan env.Redacted
		want string
	}{
		{name: "nothing moved", plan: env.Redacted{Offset: 20, Entries: []env.Entry{unchanged}, Owned: []env.OwnedEntry{kept}}, want: ""},
		{name: "one port", plan: env.Redacted{Offset: 20, Entries: []env.Entry{rewrite, unchanged}}, want: "1 port settled (offset +20)"},
		{name: "several ports", plan: env.Redacted{Offset: 10, Entries: []env.Entry{rewrite, rewrite}}, want: "2 ports settled (offset +10)"},
		{name: "owned values only", plan: env.Redacted{Owned: []env.OwnedEntry{written, kept, written}}, want: "2 owned values written"},
		{name: "both", plan: env.Redacted{Offset: 10, Entries: []env.Entry{rewrite}, Owned: []env.OwnedEntry{written}}, want: "1 port settled (offset +10) · 1 owned value written"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, SettlementNote(tc.plan))
		})
	}
}
