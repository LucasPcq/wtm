package prune

import (
	"fmt"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
	"github.com/LucasPcq/wtm/internal/testutil/ghtest"
)

// LUC-269: the scan read the hundred newest pull requests of the repository, so
// the picker never offered a worktree whose pull request was older.
func TestPickerOffersAWorktreeWhosePRIsOlderThanTheHundredNewest(t *testing.T) {
	fixture := newPruneFixture(t, "old-merged", "fresh")
	prs := make([]ghtest.PR, 0, 151)
	for index := range 150 {
		prs = append(prs, ghtest.PR{Number: 1000 - index, Branch: fmt.Sprintf("bulk/topic-%d", index), State: domain.PRStateMerged})
	}
	ghtest.Stub(t, ghtest.StubParams{PRs: append(prs, ghtest.PR{Number: 1, Branch: "old-merged", State: domain.PRStateMerged})})
	prompter := &flowtest.ScriptedPrompter{Sets: map[string][]string{KeySelection: {}}, Answers: map[string]string{KeyConfirm: confirmYes}}

	_, err := Run(t.Context(), Params{
		Context:   fixture.ctx,
		Request:   Request{Merged: true, Closed: true, Gone: true, NoFetch: true, BaseBranch: "main"},
		Prompter:  prompter,
		Presenter: &recorder{Recorder: &flowtest.Recorder{}},
	})
	if err != nil {
		t.Fatalf("prune: %v", err)
	}

	options := prompter.Content[KeySelection].Options
	if len(options) != 1 || options[0].Value != "old-merged" {
		t.Errorf("picker offers %+v, want old-merged alone", options)
	}
}
