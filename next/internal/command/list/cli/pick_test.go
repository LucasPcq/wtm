package cli

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/list"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestThePickerAsksAWorktreeThenWhatToDoWithIt(t *testing.T) {
	d := picking(t, world{}, list.Request{})
	d.looksLike("1_worktree")

	d.Keys("down", "down", "enter")
	d.looksLike("2_action")

	d.Keys("down", "enter")

	assert.Equal(t, form.Finished, d.Wizard.Ending())
	assert.Equal(t, []domain.FollowUp{{URL: "https://github.com/acme/acme/pull/12", Code: "list.open_pr", Message: "Open the pull request", Params: domain.Params{list.ParamNumber: "12"}}}, d.p.chosen().FollowUps)
}

func TestLeavingThePickerCancels(t *testing.T) {
	d := picking(t, world{}, list.Request{})

	d.Keys("esc")

	assert.Equal(t, form.Cancelled, d.Wizard.Ending())
}

func TestARefreshAndThePullRequestsLandingLaterKeepEachOthersReads(t *testing.T) {
	cases := map[string]bool{"refresh_first": true, "later_first": false}
	for name, refreshFirst := range cases {
		t.Run(name, func(t *testing.T) {
			p := newPicker(t, world{}, list.Request{})
			later, err := p.later(t.Context())
			require.NoError(t, err)
			refresh, err := p.refresh(t.Context())
			require.NoError(t, err)

			if refreshFirst {
				refresh()
				later()
			} else {
				later()
				refresh()
			}

			assert.True(t, p.withPRs)
			assert.Equal(t, &list.PR{Number: 12, URL: "https://github.com/acme/acme/pull/12"}, shownDetail(t, p, "feat/b").PR)
			assert.Equal(t, &list.Origin{Behind: 1, State: worktree.Behind}, shownDetail(t, p, "main").Origin, "the fetched counts are kept")
		})
	}
}

func TestThePullRequestsLandingLaterAreGivenToTheReadShown(t *testing.T) {
	p := newPicker(t, world{}, list.Request{})
	p.read.Warnings = append(p.read.Warnings, domain.Warning{Code: "list.shown", Message: "shown"})
	later, err := p.later(t.Context())
	require.NoError(t, err)

	later()

	assert.Equal(t, 12, shownDetail(t, p, "feat/b").PR.Number)
	assert.Equal(t, "shown", p.read.Warnings[0].Message, "the read shown is kept, not replaced")
}

func TestAWorktreeShownOffersWhatItsRowShowsWhenThePullRequestsLand(t *testing.T) {
	p := newPicker(t, world{}, list.Request{})
	from := list.Outcome{Items: []list.Item{{Subject: "feat/b", Status: domain.StatusDone, Detail: &list.Detail{
		Branch: "feat/b", Missing: true, PR: &list.PR{Number: 12, URL: "https://github.com/acme/acme/pull/12"}, Actions: []domain.FollowUp{{Command: "clean", Code: "list.clean"}},
	}}}}

	merged := list.WithPullRequests(p.read, from)

	p.read = merged
	shown := shownDetail(t, p, "feat/b")
	assert.False(t, shown.Missing)
	var codes []string
	for _, action := range shown.Actions {
		codes = append(codes, action.Code)
	}
	assert.Equal(t, []string{"list.go", "list.open_pr", "list.run_up", "list.run_down", "list.logs", "list.clean"}, codes)
}

func TestAPickSurvivesAReadThatNoLongerHasTheWorktree(t *testing.T) {
	d := picking(t, world{}, list.Request{})
	d.Keys("down", "down", "enter")

	d.p.read = list.Outcome{}
	d.Keys("enter")

	assert.Equal(t, form.Finished, d.Wizard.Ending())
	assert.Equal(t, []string{"wtm", "go", "feat/b"}, formtest.CommandLine(d.p.chosen().FollowUps[0]))
}

func TestThePickerNamesEachWorktreeItCouldNotRead(t *testing.T) {
	d := picking(t, world{Fail: map[string]error{"Dirty": errors.New("git status failed")}}, list.Request{})

	assert.Contains(t, d.Screen(), "✗ couldn't read /repo/main: git status failed")
}
