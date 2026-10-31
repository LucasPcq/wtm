package checkout_test

import (
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/checkout"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestThePullRequestIsRequiredAndANumberIsAllItTakes(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		want   int
		code   string
	}{
		{"number", []string{"42"}, 42, ""},
		{"none", []string{""}, 0, ""},
		{"word", []string{"abc"}, 0, checkout.CodePRInvalid},
		{"zero", []string{"0"}, 0, checkout.CodePRInvalid},
		{"negative", []string{"-3"}, 0, checkout.CodePRInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := checkout.Request{}.Answer(checkout.PathPR, tt.values)

			assert.Equal(t, tt.want, req.PR)
			if tt.code == "" {
				assert.NoError(t, err)
				return
			}
			assert.Equal(t, map[string]string{checkout.PathPR: tt.code}, problems(err))
		})
	}
}

func TestTheRequestTakesBackWhatItsFormAnswers(t *testing.T) {
	w := newWorld(t)
	req := checkout.Request{PR: prBehind, From: "feat/local", Mine: true}
	req.Remember = []string{domain.PathEnvFrom}

	form := w.questions(t, req).Form()
	folded, err := domain.Fold(checkout.Request{}, form.Answers())

	require.NoError(t, err)
	assert.Equal(t, prBehind, folded.PR)
	assert.Equal(t, "feat/local", folded.From)
	assert.True(t, folded.Mine)
	assert.Equal(t, []string{domain.PathEnvFrom}, folded.Remember)
}

func TestTheOpenPullRequestsAreTheOptionsAForkOrOneCheckedOutTagged(t *testing.T) {
	w := newWorld(t)

	options := w.questions(t, checkout.Request{}).PR.Options

	tags := map[string]domain.OptionTag{}
	for _, option := range options {
		require.NotNil(t, option.PR)
		tags[option.Value] = option.Tag
	}
	assert.Len(t, options, 11, "a merged pull request is no option")
	assert.Equal(t, domain.TagFork, tags["5"], "a fork named like a branch a worktree holds is a fork first")
	assert.Equal(t, domain.TagCheckedOut, tags["6"])
	assert.Empty(t, tags["1"])
}

func TestAnOptionWhoseBranchIsLocalSaysWhereItStandsAgainstOrigin(t *testing.T) {
	w := newWorld(t)

	options := w.questions(t, checkout.Request{PR: prBehind}).PR.Options

	origins := map[string]*worktree.Origin{}
	for _, option := range options {
		origins[option.Value] = option.Origin
	}
	assert.Equal(t, &worktree.Origin{Behind: 1, State: worktree.Behind}, origins["3"])
	assert.Equal(t, &worktree.Origin{Ahead: 1, Behind: 1, State: worktree.Diverged}, origins["4"])
	assert.Nil(t, origins["1"], "only origin has it")
}

func TestAnOptionThatCannotBeCheckedOutCarriesTheProblemPickingItRaises(t *testing.T) {
	w := newWorld(t)

	options := w.questions(t, checkout.Request{}).PR.Options

	problems := map[string]*domain.FieldError{}
	for _, option := range options {
		problems[option.Value] = option.Problem
	}
	require.NotNil(t, problems["5"])
	assert.Equal(t, checkout.CodeFork, problems["5"].Code)
	assert.Equal(t, "gh pr checkout 5", problems["5"].Params["command"])
	require.NotNil(t, problems["6"])
	assert.Equal(t, checkout.CodeCheckedOutElsewhere, problems["6"].Code)
	assert.Equal(t, filepath.Join(w.root, "wt-held"), problems["6"].Params["path"])
	require.NotNil(t, problems["6"].FollowUp)
	assert.Equal(t, checkout.CodeJumpIn, problems["6"].FollowUp.Code)
	assert.Nil(t, problems["1"])
}

func TestAPullRequestNoLongerOpenAskedByItsNumberIsAmongTheOptions(t *testing.T) {
	w := newWorld(t)

	questions := w.questions(t, checkout.Request{PR: prMerged})

	values := make([]string, 0, len(questions.PR.Options))
	for _, option := range questions.PR.Options {
		values = append(values, option.Value)
	}
	assert.Contains(t, values, "10")
	assert.Empty(t, questions.Errors)
}

func TestTheFiltersOfThePullRequestsAreAskedOfGitHubAndExclusive(t *testing.T) {
	w := newWorld(t)

	w.questions(t, checkout.Request{Mine: true})
	w.questions(t, checkout.Request{Review: true})
	both := w.questions(t, checkout.Request{PR: prRemote, Mine: true, Review: true})

	assert.Equal(t, []string{"list mine", "list review_requested", "view 1", "list mine"}, *w.asked)
	assert.Equal(t, map[string]string{checkout.PathReview: checkout.CodeFiltersExclusive}, problemsOf(both))
}

func TestTheParentIsTheBranchThePullRequestMergesInto(t *testing.T) {
	tests := []struct {
		name   string
		pr     int
		want   string
		pinned string
	}{
		{"local", prRemote, "main", "main"},
		{"only_on_origin", prOnBase, "origin/release", "origin/release"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newWorld(t)

			questions := w.questions(t, checkout.Request{PR: tt.pr})

			assert.Equal(t, domain.Answer[string]{Value: tt.want, Origin: domain.OriginPullRequest}, questions.From.Answer)
			assert.Equal(t, tt.pinned, questions.From.Pinned)
			assert.Empty(t, questions.Errors)
		})
	}
}

// B2: legacy recorded a base that exists nowhere as the parent, while --from refused the same name.
func TestABaseNeitherLocalNorOnOriginLeavesTheParentToBeNamed(t *testing.T) {
	w := newWorld(t)
	prs := pullRequests()
	prs[0].Base = "nowhere"
	w.usecase.PullRequest = github{prs: prs, asked: w.asked}.pullRequest

	_, err := checkout.Resolve(w.facts(t, checkout.Request{PR: prRemote}), checkout.Request{PR: prRemote})

	assert.Equal(t, map[string]string{checkout.PathFrom: domain.CodeRequired}, problems(err))
}

// B1: legacy recorded the pull request's own branch as its parent, and offered it.
func TestTheBranchOfThePullRequestIsNeverItsOwnParent(t *testing.T) {
	w := newWorld(t)
	facts := w.facts(t, checkout.Request{PR: prBehind})

	_, local := checkout.Resolve(facts, checkout.Request{PR: prBehind, From: "feat/behind"})
	_, remote := checkout.Resolve(facts, checkout.Request{PR: prBehind, From: "origin/feat/behind"})
	options := checkout.NewQuestions(facts, checkout.Request{PR: prBehind}).From.Options

	assert.Equal(t, map[string]string{checkout.PathFrom: domain.CodeSelfParent}, problems(local))
	assert.Equal(t, map[string]string{checkout.PathFrom: domain.CodeSelfParent}, problems(remote))
	for _, option := range options {
		assert.NotContains(t, []string{"feat/behind", "origin/feat/behind"}, option.Value)
	}
}

func TestAParentThatIsNoBranchIsNotFound(t *testing.T) {
	w := newWorld(t)

	_, err := checkout.Resolve(w.facts(t, checkout.Request{PR: prRemote}), checkout.Request{PR: prRemote, From: "nope"})

	assert.Equal(t, map[string]string{checkout.PathFrom: domain.CodeNotFound}, problems(err))
}

func TestEachPullRequestThatCannotBeCheckedOutIsRefusedForWhatStandsInItsWay(t *testing.T) {
	tests := []struct {
		name string
		pr   int
		code string
	}{
		{"none", 0, domain.CodeRequired},
		{"unknown", prNone, domain.CodeNotFound},
		{"fork", prFork, checkout.CodeFork},
		{"held", prHeld, checkout.CodeCheckedOutElsewhere},
		{"nowhere", prNowhere, checkout.CodeHeadMissing},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newWorld(t)
			req := checkout.Request{PR: tt.pr}

			_, err := checkout.Resolve(w.facts(t, req), req)

			assert.Equal(t, map[string]string{checkout.PathPR: tt.code}, problems(err))
		})
	}
}

func TestABranchCheckedOutElsewhereIsFollowedByTheStepIntoIt(t *testing.T) {
	w := newWorld(t)
	req := checkout.Request{PR: prHeld}

	questions := checkout.NewQuestions(w.facts(t, req), req)

	require.Len(t, questions.Errors, 1)
	require.NotNil(t, questions.Errors[0].FollowUp)
	assert.Equal(t, domain.FollowUp{Command: domain.CommandGo, Answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{"feat/held"}}}, Code: checkout.CodeJumpIn, Message: "jump in"}, *questions.Errors[0].FollowUp)
}

func TestTheFastForwardIsAskedOnlyOfALocalBranchBehindOrigin(t *testing.T) {
	tests := []struct {
		name string
		pr   int
		skip domain.Skip
	}{
		{"behind", prBehind, ""},
		{"in_sync", prLocal, domain.SkipNotBehind},
		{"diverged", prDiverged, domain.SkipNotBehind},
		{"from_origin", prRemote, domain.SkipNotBehind},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newWorld(t)

			questions := w.questions(t, checkout.Request{PR: tt.pr})

			assert.Equal(t, tt.skip, questions.FastForward.Skip)
		})
	}
}

// B3: legacy asked the fast-forward from the refs as last fetched when the pull request was picked.
func TestTheBranchIsFetchedBeforeItIsReadAgainstOrigin(t *testing.T) {
	w := newWorld(t)

	questions := w.questions(t, checkout.Request{PR: prFresh})

	assert.Empty(t, questions.FastForward.Skip, "origin moved on since the last fetch: the branch is behind")
	assert.Equal(t, []string{"feat/fresh"}, questions.Behind)
}

func TestAPullRequestGitHubCannotBeAskedOfRefusesTheQuestions(t *testing.T) {
	w := newWorld(t)
	w.usecase.PullRequests = github{asked: w.asked, err: worktree.ErrPullRequestsUnavailable}.open

	_, err := w.usecase.Questions(t.Context(), checkout.Request{})

	failure := failureOf(t, err)
	assert.Equal(t, domain.KindPrecondition, failure.Kind)
	assert.Equal(t, checkout.CodeGitHubUnavailable, failure.Code)
}
