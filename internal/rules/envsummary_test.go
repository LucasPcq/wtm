package rules

import (
	"strconv"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

// planWithRewrites builds a plan whose n links each shift one value.
func planWithRewrites(t *testing.T, n int) domain.EnvPortPlan {
	t.Helper()

	links := make([]domain.EnvPortLink, 0, n)
	bases := make(map[domain.PortRef]int, n)
	var content strings.Builder
	for i := range n {
		key := "K" + strconv.Itoa(i)
		links = append(links, domain.EnvPortLink{File: ".env", Key: key, Job: "svc", Port: key})
		bases[domain.PortRef{Job: "svc", Name: key}] = 5000 + i
		content.WriteString(key + "=http://localhost:" + strconv.Itoa(5000+i) + "\n")
	}

	plan := PlanEnvPorts(PlanEnvPortsParams{
		Links:  links,
		Bases:  bases,
		Offset: 10,
		Lines:  map[string][]domain.EnvLine{".env": ParseEnv(content.String())},
	})
	if got := len(EnvPortRewrites(plan)); got != n {
		t.Fatalf("planWithRewrites(%d) produced %d rewrites", n, got)
	}
	plan.Applied = true
	return plan
}

// declined is a plan the user refused: it still holds its rewrites, because the
// links keep normalizing the diff, but nothing was written.
func declined(plan domain.EnvPortPlan) domain.EnvPortPlan {
	plan.Applied = false
	return plan
}

// The port pass writes to the same .env as the reconciliation, so a run that
// shifted a port and changed nothing else has written changes. Reporting "no
// changes written" there is a false report, not a wording preference.
func TestEnvOutcomeSummaryCountsThePortPass(t *testing.T) {
	applied := domain.EnvFileResult{Target: ".env", Applied: true}
	untouched := domain.EnvFileResult{Target: ".env"}

	cases := []struct {
		name   string
		result domain.EnvSyncResult
		want   string
		done   bool
	}{
		{
			"ports only",
			domain.EnvSyncResult{Files: []domain.EnvFileResult{untouched}, Ports: planWithRewrites(t, 2)},
			"Settled 2 linked .env value(s).", true,
		},
		{
			"files only",
			domain.EnvSyncResult{Files: []domain.EnvFileResult{applied}},
			"Reconciled 1 file(s).", true,
		},
		{
			"both",
			domain.EnvSyncResult{Files: []domain.EnvFileResult{applied}, Ports: planWithRewrites(t, 2)},
			"Reconciled 1 file(s) and settled 2 linked value(s).", true,
		},
		{
			"neither",
			domain.EnvSyncResult{Files: []domain.EnvFileResult{untouched}},
			"No changes written.", false,
		},
		{
			"port pass declined counts as nothing written",
			domain.EnvSyncResult{Files: []domain.EnvFileResult{untouched}, Ports: declined(planWithRewrites(t, 2))},
			"No changes written.", false,
		},
		{
			"port pass declined alongside a real file change",
			domain.EnvSyncResult{Files: []domain.EnvFileResult{applied}, Ports: declined(planWithRewrites(t, 2))},
			"Reconciled 1 file(s).", true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := EnvOutcomeSummary(c.result)
			if got.Text != c.want || (got.Verdict == domain.EnvVerdictDone) != c.done {
				t.Errorf("EnvOutcomeSummary() = %+v, want {%q done=%v}", got, c.want, c.done)
			}
		})
	}
}

func TestEnvOutcomeSummaryCheckMode(t *testing.T) {
	clean := domain.EnvSyncResult{Check: true, Files: []domain.EnvFileResult{{Target: ".env"}}}
	if got := EnvOutcomeSummary(clean); got.Text != "No drift." || got.Verdict != domain.EnvVerdictDone {
		t.Errorf("EnvOutcomeSummary() = %+v, want a clean verdict", got)
	}

	// --check must not answer "no drift" about a worktree whose .env still points
	// at another worktree's services.
	drifting := domain.EnvSyncResult{Check: true, Files: []domain.EnvFileResult{{Target: ".env"}}, Ports: planWithRewrites(t, 1)}
	// Drift found is not "nothing to do": it reads in the register that says the
	// reader has something left to act on.
	if got := EnvOutcomeSummary(drifting); got.Verdict != domain.EnvVerdictAttention {
		t.Errorf("EnvOutcomeSummary() = %+v, want drift reported for a pending port shift", got)
	}
}

func TestEnvPortPlanTouches(t *testing.T) {
	plan := planWithRewrites(t, 1)
	if !EnvPortPlanTouches(plan, ".env") {
		t.Error("EnvPortPlanTouches() = false for the file it rewrites")
	}
	if EnvPortPlanTouches(plan, "apps/web/.env") {
		t.Error("EnvPortPlanTouches() = true for a file it never names")
	}
}

// A file the repository does not have anywhere is never "no drift": nothing is
// missing from it because nothing can ever be in it.
func TestEnvSummaryRefusesToCallAnUnresolvableFileClean(t *testing.T) {
	summary := EnvOutcomeSummary(domain.EnvSyncResult{
		Check: true,
		Files: []domain.EnvFileResult{
			{Target: "apps/api/.env", Unresolvable: true},
			{Target: "apps/web/.env"},
		},
	})

	if summary.Verdict == domain.EnvVerdictDone {
		t.Error("summary reported the run as done over a file that exists nowhere")
	}
	if !strings.Contains(summary.Text, "1") {
		t.Errorf("summary = %q, want it to count the unresolvable file", summary.Text)
	}
}
