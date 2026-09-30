package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/LucasPcq/wtm/internal/domain"
)

func sampleForest() domain.Forest {
	return domain.Forest{
		Roots: []domain.TreeNode{
			{
				Branch: "main", IsMain: true,
				Children: []domain.TreeNode{
					{
						Branch: "feat-auth",
						Status: domain.TreeNodeStatus{CommitsAhead: 3, PR: &domain.WorktreeListPR{Number: 123, State: domain.PRStateOpen}},
						Children: []domain.TreeNode{
							{Branch: "feat-auth-ui", Status: domain.TreeNodeStatus{CommitsAhead: 1, NeedsSync: true}},
							{Branch: "feat-auth-api", Status: domain.TreeNodeStatus{IsDirty: true}},
						},
					},
					{Branch: "feat-billing", Status: domain.TreeNodeStatus{CommitsAhead: 5}},
				},
			},
			{
				Branch: "dev", IsVirtual: true,
				Children: []domain.TreeNode{
					{Branch: "spike-cache", Status: domain.TreeNodeStatus{CommitsAhead: 2}},
				},
			},
		},
	}
}

func TestFormatTreeEmpty(t *testing.T) {
	if got := ansi.Strip(FormatTree(domain.Forest{})); got != UnchangedLine(domain.NoWorktreesMessage) {
		t.Errorf("unexpected output: %q", got)
	}
}

func TestFormatTreeConnectorsAndAnnotations(t *testing.T) {
	got := FormatTree(sampleForest())

	for _, want := range []string{
		"main",
		"├─ ", "└─ ", "│  ",
		"feat-auth", "PR #123", "↑3",
		"⚠ needs sync",
		"⚠ dirty",
		"(no worktree)",
		"↑2",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected tree output to contain %q\n---\n%s", want, got)
		}
	}
}

func TestFormatTreeChildOrderPreserved(t *testing.T) {
	got := FormatTree(sampleForest())
	ui := strings.Index(got, "feat-auth-ui")
	api := strings.Index(got, "feat-auth-api")
	if ui == -1 || api == -1 || ui > api {
		t.Fatalf("expected feat-auth-ui before feat-auth-api\n%s", got)
	}
}

func TestWriteTreeJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteTreeJSON(&buf, sampleForest()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{`"roots"`, `"branch": "feat-auth"`, `"needs_sync": true`, `"number": 123`} {
		if !strings.Contains(out, want) {
			t.Errorf("expected JSON to contain %q\n%s", want, out)
		}
	}
}

func TestWriteTreeMermaid(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteTreeMermaid(&buf, sampleForest()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"flowchart TD",
		"main --> feat_auth",
		"feat_auth --> feat_auth_ui",
		"dev --> spike_cache",
		"⚠ needs sync",
		"(no worktree)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected mermaid to contain %q\n%s", want, out)
		}
	}
}

func TestFormatTreePRStates(t *testing.T) {
	merged := formatTreePR(&domain.WorktreeListPR{Number: 9, State: domain.PRStateMerged})
	if !strings.Contains(merged, "PR #9 merged") {
		t.Errorf("expected merged label, got %q", merged)
	}
	closed := formatTreePR(&domain.WorktreeListPR{Number: 8, State: domain.PRStateClosed})
	if !strings.Contains(closed, "PR #8 closed") {
		t.Errorf("expected closed label, got %q", closed)
	}
	if formatTreePR(nil) != "" {
		t.Errorf("expected empty for nil PR")
	}
}
