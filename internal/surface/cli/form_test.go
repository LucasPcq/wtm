package cli

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LucasPcq/wtm/internal/dispatch"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/surface/cli/render"
)

func TestAFollowUpConfirmedRunsThroughTheFormItsRequestFilledIn(t *testing.T) {
	var asked, offered []string
	w := newWorld(t, worldParams{Form: applyingForm(&asked), Confirm: answering(&offered, true)})

	got := w.run(t, runParams{Args: []string{commandName, "feat/a"}, TTY: true})

	assert.Equal(t, domain.ExitCodeOK, got.Code, got.Stderr)
	assert.Equal(t, []string{"wtm make feat/next --ff", "wtm make feat/next --ff"}, offered, "the follow-up's own run proposes it again, declined")
	assert.Equal(t, []string{`{"branches":["feat/a"]}`, `{"branches":["feat/next"],"fast_forward":true}`}, asked)
	assert.Equal(t, []string{"feat/a", "feat/next"}, w.Repo.listed())
}

func TestAFollowUpDeclinedRunsNothing(t *testing.T) {
	var asked, offered []string
	w := newWorld(t, worldParams{Form: applyingForm(&asked), Confirm: answering(&offered, false)})

	got := w.run(t, runParams{Args: []string{commandName, "feat/a"}, TTY: true})

	assert.Equal(t, domain.ExitCodeOK, got.Code)
	assert.Len(t, offered, 1)
	assert.Equal(t, []string{"feat/a"}, w.Repo.listed())
	assert.Contains(t, got.Stdout, "wtm make feat/next --ff", "the follow-up stays the line the conclusion prints")
}

func TestAnUnattendedRunOffersNoFollowUp(t *testing.T) {
	var asked, offered []string
	w := newWorld(t, worldParams{Form: applyingForm(&asked), Confirm: answering(&offered, true)})

	w.run(t, runParams{Args: []string{commandName, "feat/a", "--yes"}, TTY: true})

	assert.Empty(t, offered)
	assert.Equal(t, []string{"feat/a"}, w.Repo.listed())
}

func TestAQueryNeverGoesThroughTheForm(t *testing.T) {
	var asked []string
	w := newWorld(t, worldParams{Form: applyingForm(&asked)})

	got := w.run(t, runParams{Args: []string{queryName}, TTY: true})

	assert.Equal(t, domain.ExitCodeOK, got.Code, got.Stderr)
	assert.Empty(t, asked)
}

func TestTheFormDrawsThePlanAsTheCommandsDetailDoes(t *testing.T) {
	var detail render.Detail
	form := func(_ context.Context, params FormParams) (dispatch.Reply, error) {
		detail = params.Detail
		return nil, kernel.Cancelled(kernel.Problem{Code: kernel.CodeInterrupted})
	}
	w := newWorld(t, worldParams{Form: form, Detail: pathDetail{}})

	w.run(t, runParams{Args: []string{commandName, "feat/a"}, TTY: true})

	assert.Equal(t, pathDetail{}, detail)
	assert.Contains(t, planOf(FormParams{Detail: detail}, json.RawMessage(`{"branches":["feat/a"]}`)), "will make 1 worktrees")
}
