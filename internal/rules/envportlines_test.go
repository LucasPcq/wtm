package rules

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func samplePlan(t *testing.T) domain.EnvPortPlan {
	t.Helper()
	return PlanEnvPorts(PlanEnvPortsParams{
		Links: []domain.EnvPortLink{
			{File: ".env", Key: "DATABASE_URL", Job: "svc", Port: "POSTGRES_PORT"},
			{File: ".env", Key: "REDIS_URL", Job: "svc", Port: "REDIS_PORT"},
			{File: "apps/web/.env", Key: "VITE_API_URL", Job: "svc", Port: "API_PORT"},
		},
		Bases:  map[domain.PortRef]int{{Job: "svc", Name: "POSTGRES_PORT"}: 5432, {Job: "svc", Name: "REDIS_PORT"}: 6379, {Job: "svc", Name: "API_PORT"}: 3000},
		Offset: 10,
		Lines: map[string][]domain.EnvLine{
			".env":          ParseEnv("DATABASE_URL=postgres://user:motdepasse@localhost:5432/app\nREDIS_URL=redis://localhost:6379\n"),
			"apps/web/.env": ParseEnv("VITE_API_URL=http://localhost:3000\n"),
		},
	})
}

func TestEnvPortTableLinesGroupsFilesUnderNamedRules(t *testing.T) {
	got := strings.Join(EnvPortTableLines(EnvPortTableParams{Plan: samplePlan(t)}), "\n")
	want := strings.Join([]string{
		"KEY           FOLLOWS        PORT         BECOMES",
		"── .env ────────────────────────────────────────────────────────",
		"DATABASE_URL  POSTGRES_PORT  5432 → 5442  …@localhost:5442/app",
		"REDIS_URL     REDIS_PORT     6379 → 6389  redis://localhost:6389",
		"",
		"── apps/web/.env ───────────────────────────────────────────────",
		"VITE_API_URL  API_PORT       3000 → 3010  http://localhost:3010",
	}, "\n")

	if got != want {
		t.Errorf("EnvPortTableLines() =\n%s\nwant\n%s", got, want)
	}
}

// The header labels take part in the column widths, or a short column slides out
// from under the label naming it. Compared in runes: the arrow is three bytes and
// one column, so byte offsets would disagree for the wrong reason.
func TestEnvPortTableLinesWidensColumnsToFitTheHeader(t *testing.T) {
	plan := PlanEnvPorts(PlanEnvPortsParams{
		Links:  []domain.EnvPortLink{{File: ".env", Key: "DB", Job: "svc", Port: "P"}},
		Bases:  map[domain.PortRef]int{{Job: "svc", Name: "P"}: 5432},
		Offset: 10,
		Lines:  map[string][]domain.EnvLine{".env": ParseEnv("DB=http://x:5432\n")},
	})

	lines := EnvPortTableLines(EnvPortTableParams{Plan: plan})
	header, row := lines[0], lines[len(lines)-1]
	if runeIndex(header, domain.EnvPortHeaderBecomes) != runeIndex(row, "http://") {
		t.Errorf("header and row columns do not line up:\n%s\n%s", header, row)
	}
}

func runeIndex(s, substr string) int {
	at := strings.Index(s, substr)
	if at < 0 {
		return -1
	}
	return len([]rune(s[:at]))
}

// A rule is filled out to the table's own width, never past it: the report sits
// inside a callout that would otherwise be stretched by the divider alone.
func TestEnvPortTableLinesRuleMatchesTheWidestRow(t *testing.T) {
	lines := EnvPortTableLines(EnvPortTableParams{Plan: samplePlan(t)})

	widest := 0
	for _, l := range lines {
		widest = max(widest, len([]rune(l)))
	}
	for _, l := range lines {
		if strings.HasPrefix(l, "── ") && len([]rune(l)) != widest {
			t.Errorf("rule %q is %d wide, want %d", l, len([]rune(l)), widest)
		}
	}
}

func TestEnvPortTableLinesNeverPrintsCredentials(t *testing.T) {
	for _, line := range EnvPortTableLines(EnvPortTableParams{Plan: samplePlan(t)}) {
		if strings.Contains(line, "motdepasse") {
			t.Fatalf("table line leaked a password: %q", line)
		}
	}
}

func TestEnvPortTableLinesEmptyPlanRendersNothing(t *testing.T) {
	if lines := EnvPortTableLines(EnvPortTableParams{Plan: domain.EnvPortPlan{}}); lines != nil {
		t.Errorf("EnvPortTableLines() = %v, want nil", lines)
	}
}

func TestEnvPortAnomalyLinesNamesEachRefusal(t *testing.T) {
	plan := PlanEnvPorts(PlanEnvPortsParams{
		Links: []domain.EnvPortLink{
			{File: ".env", Key: "MISSING", Job: "svc", Port: "DB_PORT"},
			{File: ".env", Key: "TWICE", Job: "svc", Port: "API_PORT"},
			{File: ".env", Key: "ELSEWHERE", Job: "svc", Port: "DB_PORT"},
		},
		Bases:  map[domain.PortRef]int{{Job: "svc", Name: "DB_PORT"}: 5432, {Job: "svc", Name: "API_PORT"}: 3000},
		Offset: 10,
		Lines: map[string][]domain.EnvLine{
			".env": ParseEnv("TWICE=http://a:3000/b:3000\nELSEWHERE=postgres://localhost:6000/app\n"),
		},
	})

	joined := strings.Join(EnvPortAnomalyLines(plan), "\n")
	for _, key := range []string{"MISSING", "TWICE", "ELSEWHERE"} {
		if !strings.Contains(joined, key) {
			t.Errorf("EnvPortAnomalyLines() never names %s:\n%s", key, joined)
		}
	}
	if !strings.Contains(joined, "── .env ") {
		t.Errorf("EnvPortAnomalyLines() does not group by file:\n%s", joined)
	}
	// The port is its own column, so no reason may repeat it — that is what kept
	// the block from overflowing the terminal.
	if strings.Contains(joined, "port DB_PORT") {
		t.Errorf("a reason repeats the port column:\n%s", joined)
	}
}

// The prompt and the recap share these lines, so what the user is asked to
// confirm reads exactly like what they are told was written. The base is on the
// line because it answers "why this key?" before, and "what moves?" after.
func TestEnvPortLinkLines(t *testing.T) {
	got := strings.Join(EnvPortLinkLines(
		[]domain.EnvPortLink{
			{File: ".env", Key: "DATABASE_URL", Job: "svc", Port: "POSTGRES_PORT"},
			{File: "apps/web/.env", Key: "REDIS_URL", Job: "svc", Port: "REDIS_PORT"},
		},
		map[domain.PortRef]int{{Job: "svc", Name: "POSTGRES_PORT"}: 5432, {Job: "svc", Name: "REDIS_PORT"}: 6379},
	), "\n")

	want := strings.Join([]string{
		".env · DATABASE_URL         follows svc.POSTGRES_PORT (5432)",
		"apps/web/.env · REDIS_URL   follows svc.REDIS_PORT (6379)",
	}, "\n")

	if got != want {
		t.Errorf("EnvPortLinkLines() =\n%s\nwant\n%s", got, want)
	}
}

func TestEnvPortLinkLinesEmpty(t *testing.T) {
	if lines := EnvPortLinkLines(nil, nil); len(lines) != 0 {
		t.Errorf("EnvPortLinkLines() = %v, want none", lines)
	}
}

// A named origin runs past fifty characters. Cut to a fixed budget it read as
// an ellipsis with a worktree in it, on a terminal with room to spare.
func namedOriginPlan(t *testing.T) domain.EnvPortPlan {
	t.Helper()
	job := domain.JobConfig{
		Name: "admin", Kind: domain.JobKindService,
		Ports: map[string]int{"ADMIN_PORT": 3000},
		URL:   &domain.JobURLConfig{Port: "ADMIN_PORT"},
	}
	return PlanEnvPorts(PlanEnvPortsParams{
		Links:  []domain.EnvPortLink{{File: ".env", Key: "VITE_ADMIN_URL", Job: "admin", Port: "ADMIN_PORT"}},
		Bases:  map[domain.PortRef]int{{Job: "admin", Name: "ADMIN_PORT"}: 3000},
		Offset: 10,
		Lines:  map[string][]domain.EnvLine{".env": ParseEnv("VITE_ADMIN_URL=http://localhost:3000\n")},
		Origins: OriginContext{
			Addressing: domain.AddressingNames,
			Jobs:       map[string]domain.JobConfig{"admin": job},
			Worktree:   "feat-x", Project: "monorepo-exemple-wtm", PublicPort: 11080,
		},
	})
}

func TestEnvPortTableLinesSpendsTheWidthItIsGivenOnTheValue(t *testing.T) {
	const origin = "http://admin.feat-x.monorepo-exemple-wtm.localhost:11080"

	wide := strings.Join(EnvPortTableLines(EnvPortTableParams{Plan: namedOriginPlan(t), Width: 120}), "\n")
	if !strings.Contains(wide, origin) {
		t.Errorf("wide table =\n%s\nwant the whole origin, unelided", wide)
	}

	// Unmeasured stays as it was: a pipe and a test read the same table they
	// always did.
	narrow := strings.Join(EnvPortTableLines(EnvPortTableParams{Plan: namedOriginPlan(t)}), "\n")
	if strings.Contains(narrow, origin) {
		t.Errorf("unmeasured table =\n%s\nwant the value elided to its default", narrow)
	}
}

func TestEnvPortTableLinesKeepsTheValueRecognisableOnANarrowSurface(t *testing.T) {
	lines := EnvPortTableLines(EnvPortTableParams{Plan: namedOriginPlan(t), Width: 40})

	// The three fixed columns already exceed 40 here. The value keeps its floor
	// rather than being cut to nothing: a wrapped line says more than an
	// elided one that says nothing.
	for _, line := range lines[1:] {
		if strings.Contains(line, "http") && !strings.Contains(line, "http://admin.feat") {
			t.Errorf("line = %q, want a value a reader can still recognise", line)
		}
	}
}
