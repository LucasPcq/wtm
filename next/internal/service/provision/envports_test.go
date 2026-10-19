package provision

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
	}, anomalyLines(plan))
}

func TestAnAnomalyLineNeverNamesAForeignHost(t *testing.T) {
	plan := env.Redact(env.Plan{Entries: []env.Entry{{File: ".env", Key: "API", Port: "P", Status: env.StatusForeignHost, ForeignHost: "u:pw@api.staging.example.com"}}})

	joined := strings.Join(anomalyLines(plan), "\n")

	assert.Contains(t, joined, "points at another host, which no job here serves")
	assert.NotContains(t, joined, "staging")
}

func TestAPlanWithNoRefusalHasNoAnomalyLine(t *testing.T) {
	assert.Empty(t, anomalyLines(env.Redacted{Entries: []env.Entry{{File: ".env", Key: "PORT", Status: env.StatusRewrite}}}))
}

func TestThePortsLeftAloneAreOneWarningCountingThem(t *testing.T) {
	plan := env.Redacted{Entries: []env.Entry{{File: ".env", Key: "DB", Port: "pg", Status: env.StatusMissingKey}}}

	warnings := portsLeftAlone(plan)

	assert.Len(t, warnings, 1)
	assert.Equal(t, "provision.env_ports_left_alone", warnings[0].Code)
	assert.Equal(t, "1", warnings[0].Params["count"])
	assert.True(t, strings.HasPrefix(warnings[0].Message, "Env ports left alone\n── .env "))
}
