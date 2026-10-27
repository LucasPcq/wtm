package reparent_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/reparent"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheWorktreesOfferedAreThoseThatCanMoveEachWithItsParent(t *testing.T) {
	w := newWorld(t)

	q := w.questions(t, reparent.Request{})

	assert.Equal(t, []domain.Option{
		{Value: featA, Parent: "main"},
		{Value: featB, Parent: featA},
		{Value: featC, Parent: featB},
		{Value: featX, Parent: "main"},
	}, q.Worktrees.Options)
}

func TestAWorktreeThatCannotMoveIsAFieldErrorOfItsEntry(t *testing.T) {
	w := newWorld(t)
	writeFile(t, w.stateDir+"/config.toml", "[worktrees]\nbase_path = \"../.trees\"\nbase_branch = \""+featX+"\"\n")
	cases := map[string]struct {
		names []string
		code  string
	}{
		"blank":         {names: []string{""}, code: domain.CodeRequired},
		"given_twice":   {names: []string{featC, featC}, code: domain.CodeDistinct},
		"no_worktree":   {names: []string{"feat/nope"}, code: domain.CodeNotFound},
		"main_worktree": {names: []string{"main"}, code: reparent.CodeMainWorktree},
		"base_branch":   {names: []string{featX}, code: reparent.CodeBaseBranch},
		"detached":      {names: []string{w.at("detached")}, code: reparent.CodeDetached},
		"unrecorded":    {names: []string{raw}, code: reparent.CodeUnrecorded},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			q := w.questions(t, moving(topic, c.names...))

			at := "worktree[" + string(rune('0'+len(c.names)-1)) + "]"
			assert.Equal(t, []string{c.code}, fieldCodes(q.Errors)[at])
		})
	}
}

func TestANewParentUnderAWorktreeMovedIsACycleNamingWhichOne(t *testing.T) {
	w := newWorld(t)

	q := w.questions(t, moving(featC, featA))

	require.Len(t, q.Errors, 1)
	assert.Equal(t, reparent.CodeCycle, q.Errors[0].Code)
	assert.Equal(t, domain.PathTo, q.Errors[0].Path)
	assert.Equal(t, domain.Params{"value": featC, "under": featA}, q.Errors[0].Params)
}

func TestTheNewParentIsChecked(t *testing.T) {
	w := newWorld(t)
	cases := map[string]struct {
		to   string
		code string
	}{
		"missing":       {to: "", code: domain.CodeRequired},
		"itself":        {to: featC, code: domain.CodeSelfParent},
		"no_branch":     {to: "feat/nope", code: domain.CodeNotFound},
		"origin_bare":   {to: remoteOnly, code: domain.CodeNotFound},
		"origin_branch": {to: "origin/" + remoteOnly},
		"local_branch":  {to: topic},
		"its_parent":    {to: featB},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			q := w.questions(t, moving(c.to, featC))

			if c.code == "" {
				assert.Empty(t, q.Errors)
				return
			}
			assert.Equal(t, map[string][]string{domain.PathTo: {c.code}}, fieldCodes(q.Errors))
		})
	}
}

func TestTheParentsThatWouldCloseACycleAreRefusedRatherThanOffered(t *testing.T) {
	w := newWorld(t)

	q := w.questions(t, reparent.Request{Worktrees: []string{featA}})

	assert.Equal(t, []string{featX, "main", raw, topic, "origin/" + remoteOnly}, valuesOf(q.To.Options))
	refused := map[string]string{}
	for _, problem := range q.To.Refused {
		refused[problem.Params["value"]] = problem.Code + ":" + problem.Params["under"]
	}
	assert.Equal(t, map[string]string{featA: domain.CodeSelfParent + ":", featB: reparent.CodeCycle + ":" + featA, featC: reparent.CodeCycle + ":" + featA}, refused)
	assert.Equal(t, "main", q.To.Pinned)
}

func TestTheWorktreeTheCallerStandsInIsPickedWhenNoneIsNamed(t *testing.T) {
	w := newWorld(t)

	in := w.questions(t, reparent.Request{To: "main", Caller: domain.Caller{Cwd: w.at(featC)}})
	elsewhere := w.questions(t, reparent.Request{To: "main", Caller: domain.Caller{Cwd: w.main}})

	assert.Equal(t, []string{featC}, in.Worktrees.Picked)
	assert.Equal(t, domain.OriginDefault, in.Worktrees.Origin)
	assert.Contains(t, in.Worktrees.Options, domain.Option{Value: featC, Parent: featB, Tag: domain.TagCurrent})
	assert.Empty(t, in.Errors)
	assert.Equal(t, map[string][]string{domain.PathWorktree: {domain.CodeRequired}}, fieldCodes(elsewhere.Errors))
}

func TestTheRequestRoundTripsThroughItsForm(t *testing.T) {
	req := moving(featA, featB, featC)

	for _, field := range (reparent.Questions{}).Form().Fields {
		path := field.Spec().Path
		answered, err := reparent.Request{}.Answer(path, req.Values(path))
		require.NoError(t, err)
		assert.Equal(t, req.Values(path), answered.Values(path), path)
	}
}

func TestARecordThatDoesNotReadRefusesItsEntryWithItsCause(t *testing.T) {
	w := newWorld(t)
	writeFile(t, w.recordPath(t, featX), "{")

	q := w.questions(t, moving("main", featX))

	require.Len(t, q.Errors, 1)
	assert.Equal(t, reparent.CodeMetaUnreadable, q.Errors[0].Code)
	assert.Contains(t, q.Errors[0].Params["cause"], "unexpected end of JSON input")
	assert.NotContains(t, valuesOf(q.Worktrees.Options), featX)
}
