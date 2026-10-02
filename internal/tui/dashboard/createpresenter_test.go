package dashboard

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	createflow "github.com/LucasPcq/wtm/internal/flow/create"
)

func posted(run func(send func(tea.Msg))) []tea.Msg {
	msgs := make(chan tea.Msg, 64)
	run(func(msg tea.Msg) { msgs <- msg })
	close(msgs)
	var all []tea.Msg
	for msg := range msgs {
		all = append(all, msg)
	}
	return all
}

func outputText(msgs []tea.Msg) string {
	var lines []string
	for _, msg := range msgs {
		if line, ok := msg.(OutputLineMsg); ok {
			lines = append(lines, line.Text)
		}
	}
	return strings.Join(lines, "\n")
}

// Without a target, a stage lands on every row the run holds: the second
// branch's hooks would read as the first one's.
func TestABatchTagsEachStageWithItsBranch(t *testing.T) {
	msgs := posted(func(send func(tea.Msg)) {
		p := newCreatePresenter(presenter{send: send, id: 1})
		p.BranchStarted(createflow.BranchProgress{Branch: "feat/a", Position: 1, Total: 2})
		_ = p.Stage(flow.StageParams{Message: "creating a", Work: func() error { return nil }})
		p.BranchStarted(createflow.BranchProgress{Branch: "feat/b", Position: 2, Total: 2})
		_ = p.HookPhase(flow.HookPhaseParams{Title: "hooks b", Run: func(flow.HookSink) error { return nil }})
	})

	targets := map[string]string{}
	for _, msg := range msgs {
		if stage, ok := msg.(opStageMsg); ok {
			targets[stage.stage] = stage.target
		}
	}
	if targets["creating a"] != "feat/a" || targets["hooks b"] != "feat/b" {
		t.Errorf("stage targets = %v, want each stage on the branch being created", targets)
	}
}

func TestASingleCreateKeepsItsStagesUntargeted(t *testing.T) {
	msgs := posted(func(send func(tea.Msg)) {
		_ = newCreatePresenter(presenter{send: send, id: 1}).Stage(flow.StageParams{Message: "creating", Work: func() error { return nil }})
	})
	for _, msg := range msgs {
		if stage, ok := msg.(opStageMsg); ok && stage.target != "" {
			t.Errorf("target = %q, want none: the run holds only the one row", stage.target)
		}
	}
}

func TestABatchSelectsOnlyItsFirstCreatedBranch(t *testing.T) {
	msgs := posted(func(send func(tea.Msg)) {
		p := newCreatePresenter(presenter{send: send, id: 1})
		p.BranchCreated(domain.CreateResult{Branch: "feat/a"})
		p.BranchCreated(domain.CreateResult{Branch: "feat/b"})
	})

	var created []createdMsg
	for _, msg := range msgs {
		if c, ok := msg.(createdMsg); ok {
			created = append(created, c)
		}
	}
	if len(created) != 2 {
		t.Fatalf("created = %+v, want each branch announced as it lands", created)
	}
	if !created[0].selects || created[1].selects {
		t.Errorf("created = %+v, want the cursor moved once, to the first", created)
	}
	if body := outputText(msgs); !strings.Contains(body, "feat/a") || !strings.Contains(body, "feat/b") {
		t.Errorf("each created branch must be named in the panel:\n%s", body)
	}
}

// The run's error is ErrAborted once a batch fails, which the dashboard keeps
// quiet: this line is the only place the failure is said.
func TestABatchFailureIsNamedInThePanel(t *testing.T) {
	msgs := posted(func(send func(tea.Msg)) {
		newCreatePresenter(presenter{send: send, id: 1}).BranchFailed(domain.CreateFailure{Branch: "feat/b", Error: "path exists"})
	})
	body := outputText(msgs)
	if !strings.Contains(body, domain.GlyphFailure) || !strings.Contains(body, "feat/b") || !strings.Contains(body, "path exists") {
		t.Errorf("panel = %q, want the failed branch and its cause", body)
	}
}

func TestABatchConclusionCountsWhatHappened(t *testing.T) {
	outcome := createflow.Outcome{
		Results: []domain.CreateResult{{Branch: "feat/a"}, {Branch: "feat/c"}},
		Failed:  []domain.CreateFailure{{Branch: "feat/b", Error: "boom"}},
	}
	msgs := posted(func(send func(tea.Msg)) {
		if err := newCreatePresenter(presenter{send: send, id: 1}).Created(outcome); err != nil {
			t.Fatal(err)
		}
	})
	body := outputText(msgs)
	if !strings.Contains(body, "2 "+domain.TallyCreated) || !strings.Contains(body, "1 "+domain.TallyFailed) {
		t.Errorf("panel = %q, want a counted readout", body)
	}
	for _, msg := range msgs {
		if _, ok := msg.(createdMsg); ok {
			t.Error("a batch announced each branch already; the conclusion must not select again")
		}
	}
}

func TestASingleCreateSelectsItsWorktree(t *testing.T) {
	msgs := posted(func(send func(tea.Msg)) {
		_ = newCreatePresenter(presenter{send: send, id: 1}).Created(createflow.Outcome{Results: []domain.CreateResult{{Branch: "feat/a"}}})
	})
	found := false
	for _, msg := range msgs {
		if c, ok := msg.(createdMsg); ok && c.branch == "feat/a" && c.selects {
			found = true
		}
	}
	if !found {
		t.Error("a single create must select the worktree it made")
	}
}
