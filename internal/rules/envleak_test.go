package rules

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

const leakBase = 5432

func leakPlan(value string) domain.EnvPortPlan {
	return PlanEnvPorts(PlanEnvPortsParams{
		Links:  []domain.EnvPortLink{{File: ".env", Key: "K", Job: "db", Port: "P"}},
		Bases:  map[domain.PortRef]int{{Job: "db", Name: "P"}: leakBase},
		Offset: 10,
		Lines:  map[string][]domain.EnvLine{".env": ParseEnv("K=" + value + "\n")},
	})
}

func portPlanSurfaces(t *testing.T, plan domain.EnvPortPlan) []string {
	t.Helper()
	body, err := json.Marshal(RedactEnvPortPlan(plan))
	if err != nil {
		t.Fatal(err)
	}
	return append(append([]string{string(body)}, EnvPortTableLines(EnvPortTableParams{Plan: plan})...), EnvPortAnomalyLines(plan)...)
}

// LUC-279 audit, leak A: a host ending in ".localhost" was read from the value
// and printed, so a password of that shape came out of the port table and JSON.
func TestEnvPortReportPrintsNoHostReadFromTheValue(t *testing.T) {
	for _, value := range []string{
		"postgres://app:" + fakeSecret + ".localhost:5432/zz@db/app",
		"host=localhost password=" + fakeSecret + ".localhost:5432",
		"Server=db;Password=" + fakeSecret + ".localhost:5432;",
		"redis://:" + fakeSecret + ".localhost:5432",
	} {
		for _, line := range portPlanSurfaces(t, leakPlan(value)) {
			if strings.Contains(line, fakeSecret) {
				t.Errorf("%q: printed %q", value, line)
			}
		}
	}
}

// LUC-279 audit, leak B: a restored row paired the worktree's value with the
// source's, so a hand-edited number in a password read as a moved port.
func TestEnvRestoredRowsPrintNoNumberReadFromTheValue(t *testing.T) {
	for _, pair := range [][2]string{
		{"Server=db;Password=ab:1111;", "Server=db;Password=ab:2222;"},
		{"host=db password=ab:1111 dbname=x", "host=db password=ab:2222 dbname=x"},
		{"host=db password=x.localhost:1111", "host=db password=x.localhost:2222"},
	} {
		entries := []domain.EnvRestoredEntry{{File: ".env", Key: "K", From: pair[0], To: pair[1]}}
		lines := EnvRestoredRows(EnvRestoredRowsParams{Entries: entries, File: ".env"})
		body, _ := json.Marshal(RedactEnvResult(domain.EnvSyncResult{Restored: entries}))
		for _, line := range append(lines, string(body)) {
			if strings.Contains(line, "1111") || strings.Contains(line, "2222") || strings.Contains(line, "x.localhost") {
				t.Errorf("%q: printed %q", pair[0], line)
			}
		}
	}
}
