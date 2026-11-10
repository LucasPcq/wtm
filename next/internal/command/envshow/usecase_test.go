package envshow_test

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/envshow"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEachKeySaysWhatItFollowsItsValuesAndWhatWtmWrites(t *testing.T) {
	d, warnings := read(t, acmeSeed(), envshow.Request{Worktree: billing})

	assert.Empty(t, warnings)
	assert.Equal(t, envshow.Key{Key: "PORT", Follows: envshow.FollowsPort, Job: "web", Port: "PORT", Main: ptr("3000"), Here: ptr("3010"), Writes: ptr("3020"), Status: "in_step", OutOfStep: true}, keyOf(t, d, webEnv, "PORT"))
	assert.Equal(t, envshow.Key{Key: "API_URL", Follows: envshow.FollowsAddress, Job: "api", Port: "PORT", Main: ptr("http://localhost:4000"), Here: ptr("http://localhost:4020"), Writes: ptr("http://localhost:4020"), Status: "in_step"}, keyOf(t, d, webEnv, "API_URL"))
	assert.Equal(t, envshow.Key{Key: "MAILER_URL", Follows: envshow.FollowsNothing, Job: "stack", Port: "MAILPIT_UI", Main: ptr(""), Here: ptr(""), Withheld: true, Status: "in_step"}, keyOf(t, d, apiEnv, "MAILER_URL"))
	assert.Equal(t, envshow.Key{Key: "JWT_SECRET", Follows: envshow.FollowsCopied, Main: ptr(""), Here: ptr(""), Withheld: true, Status: "in_step"}, keyOf(t, d, apiEnv, "JWT_SECRET"))
}

func TestTheValuesAreShownWhenAsked(t *testing.T) {
	d, _ := read(t, acmeSeed(), envshow.Request{Worktree: billing, ShowValues: true})

	key := keyOf(t, d, apiEnv, "JWT_SECRET")
	assert.False(t, key.Withheld)
	assert.Equal(t, ptr("s3cret"), key.Here)
}

func TestTheKeysToCheckAreThoseFollowingNothingAndThoseOutOfStep(t *testing.T) {
	d, _ := read(t, acmeSeed(), envshow.Request{Worktree: billing})

	assert.Equal(t, []envshow.Check{
		{File: apiEnv, Key: "MAILER_URL", Kind: envshow.CheckUnlinked, Job: "stack", Port: "MAILPIT_UI"},
		{File: webEnv, Key: "PORT", Kind: envshow.CheckOutOfStep, Job: "web", Port: "PORT", Here: ptr("3010"), Writes: ptr("3020")},
	}, d.Checks)
}

func TestTheFilesAreGroupedByAppWithTheirOriginAndTheJobsRunThere(t *testing.T) {
	d, _ := read(t, acmeSeed(), envshow.Request{Worktree: billing})

	require.Len(t, d.Apps, 3)
	assert.Equal(t, "apps/api", d.Apps[0].Dir)
	assert.Equal(t, []string{"api"}, d.Apps[0].Jobs)
	assert.Equal(t, 3, d.Apps[0].Keys)
	assert.Equal(t, 1, d.Apps[0].Following)
	assert.Equal(t, 1, d.Apps[0].ToCheck)
	assert.Equal(t, envshow.OriginTemplate, d.Apps[0].Files[0].Origin)
	assert.Equal(t, envshow.File{Target: docsEnv, Name: ".env", Origin: envshow.OriginNone, Missing: true, Keys: []envshow.Key{}}, d.Apps[2].Files[0])
}

func TestTheMainCheckoutReadsItsOwnFiles(t *testing.T) {
	d, _ := read(t, acmeSeed(), envshow.Request{Worktree: "main"})

	assert.Equal(t, envshow.OriginOwn, d.Apps[0].Files[0].Origin)
	key := keyOf(t, d, webEnv, "PORT")
	assert.Equal(t, ptr("3000"), key.Main)
	assert.False(t, key.OutOfStep)
}

func TestAWorktreeYetToChooseItsIsolationSaysSoAndWhatGivesItOne(t *testing.T) {
	s := acmeSeed()
	s.Meta[billing] = worktree.Meta{SourceBranch: "main", EnvStrategy: project.EnvExample, Ordinal: 2}

	d, warnings := read(t, s, envshow.Request{Worktree: billing})

	assert.True(t, d.Pending)
	require.Len(t, warnings, 1)
	assert.Equal(t, envshow.CodePending, warnings[0].Code)
	assert.Equal(t, domain.CommandEnv, warnings[0].FollowUp.Command)
	assert.Nil(t, keyOf(t, d, webEnv, "PORT").Writes)
}

func TestAnUnreadableRunTomlSaysWhatWtmWritesIsUnknown(t *testing.T) {
	s := acmeSeed()
	s.RunErr = errors.New("run.toml: bad")

	_, warnings := read(t, s, envshow.Request{Worktree: billing})

	require.Len(t, warnings, 1)
	assert.Equal(t, envshow.CodeUnsettled, warnings[0].Code)
}
