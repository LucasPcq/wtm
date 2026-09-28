package output_test

import (
	"bytes"
	"encoding/json"
	"github.com/charmbracelet/x/ansi"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/output"
)

func decodeResults(t *testing.T, outcome runlogs.Outcome) []domain.JobActionResult {
	t.Helper()
	var buf bytes.Buffer
	if err := output.WriteRunOutcomeJSON(&buf, outcome); err != nil {
		t.Fatalf("write: %v", err)
	}
	var results []domain.JobActionResult
	if err := json.Unmarshal(buf.Bytes(), &results); err != nil {
		t.Fatalf("decode %s: %v", buf.String(), err)
	}
	return results
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
	if err := output.WriteRunOutcomeJSON(&buf, runlogs.Outcome{
		Results: []domain.JobActionResult{{Name: "seed", Status: domain.JobActionDone}},
	}); err != nil {
		t.Fatalf("write: %v", err)
	}

	if bytes.Contains(buf.Bytes(), []byte("ports")) {
		t.Errorf("a run with no probe must not emit an empty field:\n%s", buf.String())
	}
}

// The shape follows the arity (LUC-198): one worktree keeps the bare array of
// job results an agent already parses, byte for byte.
func TestRunOutcomesJSONOfOneWorktreeIsTheArrayItAlwaysWas(t *testing.T) {
	outcome := runlogs.Outcome{
		WorkDir: "/work/main",
		Results: []domain.JobActionResult{{Name: "web", Status: domain.JobActionStarted}},
	}

	var one, many bytes.Buffer
	if err := output.WriteRunOutcomeJSON(&one, outcome); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := output.WriteRunOutcomesJSON(&many, runlogs.Outcomes{outcome}); err != nil {
		t.Fatalf("write: %v", err)
	}
	if one.String() != many.String() {
		t.Errorf("a run over one worktree changed shape:\n%s\nwant\n%s", many.String(), one.String())
	}
}

func TestRunOutcomesJSONOfSeveralWorktreesNamesEachOne(t *testing.T) {
	var buf bytes.Buffer
	err := output.WriteRunOutcomesJSON(&buf, runlogs.Outcomes{
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
	if documents[0].Worktree != "main" || documents[0].Path != "/work/main" || documents[0].Aborted {
		t.Errorf("first document = %+v", documents[0])
	}
	if documents[1].Worktree != "feature" || !documents[1].Aborted {
		t.Errorf("second document = %+v, want the aborted worktree named as such", documents[1])
	}
	if len(documents[1].Jobs) != 1 || documents[1].Jobs[0].Name != "web" {
		t.Errorf("second document's jobs = %+v", documents[1].Jobs)
	}
}

func TestWorktreeJobResultsJSONFollowsTheArity(t *testing.T) {
	jobs := []domain.JobActionResult{{Name: "web", Status: domain.JobActionStopped}}

	var flat, one bytes.Buffer
	if err := output.WriteJobResultsJSON(&flat, jobs); err != nil {
		t.Fatalf("write: %v", err)
	}
	err := output.WriteWorktreeJobResultsJSON(&one, []domain.WorktreeJobResults{
		{Worktree: "main", Path: "/work/main", Jobs: jobs},
	})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if flat.String() != one.String() {
		t.Errorf("one worktree changed shape:\n%s\nwant\n%s", one.String(), flat.String())
	}

	var many bytes.Buffer
	err = output.WriteWorktreeJobResultsJSON(&many, []domain.WorktreeJobResults{
		{Worktree: "main", Path: "/work/main", Jobs: jobs},
		{Worktree: "feature", Path: "/work/feature"},
	})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	var documents []domain.WorktreeJobResults
	if err := json.Unmarshal(many.Bytes(), &documents); err != nil {
		t.Fatalf("decode %s: %v", many.String(), err)
	}
	if len(documents) != 2 || documents[1].Worktree != "feature" {
		t.Fatalf("documents = %+v, want one per worktree", documents)
	}
	if documents[1].Jobs == nil {
		t.Error("a worktree that stopped nothing came back without a job array")
	}
}

// A writer does not patch what it was handed.
func TestWorktreeJobResultsJSONDoesNotTouchItsInput(t *testing.T) {
	results := []domain.WorktreeJobResults{
		{Worktree: "main", Path: "/work/main"},
		{Worktree: "feature", Path: "/work/feature"},
	}

	if err := output.WriteWorktreeJobResultsJSON(&bytes.Buffer{}, results); err != nil {
		t.Fatalf("write: %v", err)
	}
	for _, result := range results {
		if result.Jobs != nil {
			t.Errorf("%q came back with a job slice the writer filled in", result.Worktree)
		}
	}
}

// The frame writes one blank line after the body it is given, and a body whose
// last line has no break of its own swallows it.
func TestRunDownRecapEndsOnItsOwnLineBreak(t *testing.T) {
	recap := output.FormatRunDownRecap(output.RunDownRecapParams{
		Profile: "dev",
		Results: []domain.WorktreeJobResults{
			{Worktree: "main", Path: "/work/main", Jobs: []domain.JobActionResult{
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
	recap := ansi.Strip(output.FormatRunDownRecap(output.RunDownRecapParams{
		Results: []domain.WorktreeJobResults{
			{Worktree: "feat/x", Path: "/work/x", Jobs: []domain.JobActionResult{
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
