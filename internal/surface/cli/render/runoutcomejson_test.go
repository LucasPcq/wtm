package render_test

import (
	"bytes"
	"encoding/json"
	"github.com/charmbracelet/x/ansi"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/surface/cli/render"
)

func decodeResults(t *testing.T, outcome runlogs.Outcome) []domain.JobActionResult {
	t.Helper()
	var buf bytes.Buffer
	if err := render.WriteRunOutcomesJSON(&buf, runlogs.Outcomes{outcome}); err != nil {
		t.Fatalf("write: %v", err)
	}
	var documents []domain.WorktreeRunResult
	if err := json.Unmarshal(buf.Bytes(), &documents); err != nil {
		t.Fatalf("decode %s: %v", buf.String(), err)
	}
	if len(documents) != 1 {
		t.Fatalf("documents = %d, want the one worktree", len(documents))
	}
	return documents[0].Jobs
}

func TestRunOutcomeJSONCarriesThePortVerdicts(t *testing.T) {
	// The probe costs the machine surface its whole budget; dropping the verdict
	// leaves an agent reading "started" with no way to learn the port is silent.
	results := decodeResults(t, runlogs.Outcome{
		Results: []domain.JobActionResult{{Name: "web", Status: domain.JobActionStarted}},
		Probes: []domain.PortProbe{
			{Job: "web", Name: "WEB_PORT", Port: 5183, Status: domain.PortSilent, BaseListening: 5173},
		},
	})

	if len(results) != 1 || len(results[0].Ports) != 1 {
		t.Fatalf("expected the probe on its job, got %+v", results)
	}
	probe := results[0].Ports[0]
	if probe.Status != domain.PortSilent || probe.Port != 5183 || probe.BaseListening != 5173 {
		t.Errorf("probe = %+v, want the silent verdict with its base hint", probe)
	}
}

func TestRunOutcomeJSONPutsEachProbeOnItsOwnJob(t *testing.T) {
	results := decodeResults(t, runlogs.Outcome{
		Results: []domain.JobActionResult{
			{Name: "api", Status: domain.JobActionStarted},
			{Name: "web", Status: domain.JobActionStarted},
		},
		Probes: []domain.PortProbe{
			{Job: "web", Name: "WEB_PORT", Port: 5183, Status: domain.PortListening},
		},
	})

	if len(results[0].Ports) != 0 {
		t.Errorf("api was not probed, yet carries %+v", results[0].Ports)
	}
	if len(results[1].Ports) != 1 {
		t.Errorf("web's probe did not reach it: %+v", results[1])
	}
}

func TestRunOutcomeJSONOmitsPortsWhenNothingWasProbed(t *testing.T) {
	var buf bytes.Buffer
	if err := render.WriteRunOutcomesJSON(&buf, runlogs.Outcomes{{
		Results: []domain.JobActionResult{{Name: "seed", Status: domain.JobActionDone}},
	}}); err != nil {
		t.Fatalf("write: %v", err)
	}

	if bytes.Contains(buf.Bytes(), []byte("ports")) {
		t.Errorf("a run with no probe must not emit an empty field:\n%s", buf.String())
	}
}

// One worktree is an array of one document, never the bare job array: the
// shape does not depend on how many worktrees the run reached.
func TestRunOutcomesJSONOfOneWorktreeIsStillAnArrayOfDocuments(t *testing.T) {
	var buf bytes.Buffer
	err := render.WriteRunOutcomesJSON(&buf, runlogs.Outcomes{{
		WorkDir: "/work/main", Worktree: "main", Profile: "dev",
		Results: []domain.JobActionResult{{Name: "web", Status: domain.JobActionStarted}},
	}})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	var documents []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &documents); err != nil {
		t.Fatalf("decode %s: %v", buf.String(), err)
	}
	if len(documents) != 1 || documents[0]["branch"] != "main" || documents[0]["path"] != "/work/main" {
		t.Errorf("documents = %v, want one naming main by branch and path", documents)
	}
	if _, stale := documents[0]["worktree"]; stale {
		t.Errorf("document still carries a worktree key: %v", documents[0])
	}
}

func TestRunOutcomesJSONOfNothingIsAnEmptyArray(t *testing.T) {
	var buf bytes.Buffer
	if err := render.WriteRunOutcomesJSON(&buf, nil); err != nil {
		t.Fatalf("write: %v", err)
	}
	if strings.TrimSpace(buf.String()) != "[]" {
		t.Errorf("got %q, want []", buf.String())
	}
}

func TestRunOutcomesJSONOfSeveralWorktreesNamesEachOne(t *testing.T) {
	var buf bytes.Buffer
	err := render.WriteRunOutcomesJSON(&buf, runlogs.Outcomes{
		{
			WorkDir: "/work/main", Worktree: "main", Profile: "dev",
			Results: []domain.JobActionResult{{Name: "web", Status: domain.JobActionStarted}},
		},
		{
			WorkDir: "/work/feature", Worktree: "feature", Profile: "dev",
			Failed:  "web",
			Results: []domain.JobActionResult{{Name: "web", Status: domain.JobActionError}},
		},
	})
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	var documents []domain.WorktreeRunResult
	if err := json.Unmarshal(buf.Bytes(), &documents); err != nil {
		t.Fatalf("decode %s: %v", buf.String(), err)
	}
	if len(documents) != 2 {
		t.Fatalf("documents = %d, want one per worktree", len(documents))
	}
	if documents[0].Branch != "main" || documents[0].Path != "/work/main" || documents[0].Aborted {
		t.Errorf("first document = %+v", documents[0])
	}
	if documents[1].Branch != "feature" || !documents[1].Aborted {
		t.Errorf("second document = %+v, want the aborted worktree named as such", documents[1])
	}
	if len(documents[1].Jobs) != 1 || documents[1].Jobs[0].Name != "web" {
		t.Errorf("second document's jobs = %+v", documents[1].Jobs)
	}
}

func TestWorktreeJobResultsJSONIsAnArrayOfDocumentsWhateverTheArity(t *testing.T) {
	jobs := []domain.JobActionResult{{Name: "web", Status: domain.JobActionStopped}}

	for _, results := range [][]domain.WorktreeJobResults{
		{{Branch: "main", Path: "/work/main", Jobs: jobs}},
		{{Branch: "main", Path: "/work/main", Jobs: jobs}, {Branch: "feature", Path: "/work/feature"}},
	} {
		var buf bytes.Buffer
		if err := render.WriteWorktreeJobResultsJSON(&buf, results); err != nil {
			t.Fatalf("write: %v", err)
		}
		var documents []map[string]any
		if err := json.Unmarshal(buf.Bytes(), &documents); err != nil {
			t.Fatalf("decode %s: %v", buf.String(), err)
		}
		if len(documents) != len(results) {
			t.Fatalf("documents = %d, want %d", len(documents), len(results))
		}
		last := documents[len(documents)-1]
		if last["branch"] != results[len(results)-1].Branch || last["path"] != results[len(results)-1].Path {
			t.Errorf("document = %v, want branch and path", last)
		}
		if jobs, ok := last["jobs"].([]any); !ok || jobs == nil {
			t.Errorf("document = %v, want a job array even when empty", last)
		}
	}
}

// A writer does not patch what it was handed.
func TestWorktreeJobResultsJSONDoesNotTouchItsInput(t *testing.T) {
	results := []domain.WorktreeJobResults{
		{Branch: "main", Path: "/work/main"},
		{Branch: "feature", Path: "/work/feature"},
	}

	if err := render.WriteWorktreeJobResultsJSON(&bytes.Buffer{}, results); err != nil {
		t.Fatalf("write: %v", err)
	}
	for _, result := range results {
		if result.Jobs != nil {
			t.Errorf("%q came back with a job slice the writer filled in", result.Branch)
		}
	}
}

// The frame writes one blank line after the body it is given, and a body whose
// last line has no break of its own swallows it.
func TestRunDownRecapEndsOnItsOwnLineBreak(t *testing.T) {
	recap := render.FormatRunDownRecap(render.RunDownRecapParams{
		Profile: "dev",
		Results: []domain.WorktreeJobResults{
			{Branch: "main", Path: "/work/main", Jobs: []domain.JobActionResult{
				{Name: "web", Status: domain.JobActionStopped},
			}},
		},
	})

	if !strings.HasSuffix(recap, "\n") {
		t.Errorf("recap does not end on a line break: %q", recap[max(len(recap)-40, 0):])
	}
	if strings.HasSuffix(recap, "\n\n") {
		t.Errorf("recap ends on a blank line of its own, which the frame adds: %q", recap[max(len(recap)-40, 0):])
	}
}

// A shared job another worktree still holds was let go of, not stopped: the
// recap says so apart, or the reader believes the service is gone for everyone.
func TestRunDownRecapTellsAReleasedJobApart(t *testing.T) {
	recap := ansi.Strip(render.FormatRunDownRecap(render.RunDownRecapParams{
		Results: []domain.WorktreeJobResults{
			{Branch: "feat/x", Path: "/work/x", Jobs: []domain.JobActionResult{
				{Name: "web", Status: domain.JobActionStopped},
				{Name: "postgres", Status: domain.JobActionReleased},
			}},
		},
	}))

	if !strings.Contains(recap, "Stopped:      web") || strings.Contains(recap, "Stopped:      web, postgres") {
		t.Errorf("recap = %q, want only web stopped", recap)
	}
	if !strings.Contains(recap, "Released:     postgres") {
		t.Errorf("recap = %q, want postgres released", recap)
	}
}
