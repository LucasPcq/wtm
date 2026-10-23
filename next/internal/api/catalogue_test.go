package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEachCallOpensTheUseCaseOnItsDirectory(t *testing.T) {
	greet := greet(func(_ context.Context, dir string) (useCase, error) { return useCase{dir: dir}, nil })

	first := greet.Execute(t.Context(), Call{Dir: "/a", Answers: named("x")})
	second := greet.Execute(t.Context(), Call{Dir: "/b", Answers: named("y"), Options: domain.RunOptions{Surface: "desktop"}})

	assert.Equal(t, http.StatusOK, first.Status)
	assert.Equal(t, "greet", first.Envelope.Command)
	assert.Equal(t, "/a:x:api", first.Envelope.Op)
	assert.Equal(t, "/b:y:desktop", second.Envelope.Op)
}

func TestARefusalHasTheStatusOfItsKind(t *testing.T) {
	for kind, status := range map[domain.Kind]int{
		domain.KindInvalid:      http.StatusBadRequest,
		domain.KindRefused:      http.StatusForbidden,
		domain.KindConflict:     http.StatusConflict,
		domain.KindPrecondition: http.StatusPreconditionFailed,
		domain.KindCancelled:    statusClientClosed,
		domain.KindInternal:     http.StatusInternalServerError,
		"unheard of":            http.StatusInternalServerError,
	} {
		refusing := useCase{refusal: &domain.Error{Kind: kind, Code: "greet.refused"}}
		greet := greet(func(context.Context, string) (useCase, error) { return refusing, nil })

		got := greet.Execute(t.Context(), Call{Dir: "/repo"})

		assert.Equal(t, status, got.Status, kind)
		assert.Equal(t, "greet.refused", got.Envelope.Err.Code, kind)
	}
}

func TestADirectoryThatIsNotAbsoluteIsRefusedBeforeAnythingOpens(t *testing.T) {
	opened := false
	greet := greet(func(context.Context, string) (useCase, error) { opened = true; return useCase{}, nil })

	for _, dir := range []string{"", ".", "repo/main"} {
		got := greet.Execute(t.Context(), Call{Dir: dir, Answers: named("x")})

		assert.Equal(t, http.StatusBadRequest, got.Status, dir)
		assert.Equal(t, domain.CodeInvalidRequest, got.Envelope.Err.Code, dir)
	}
	assert.False(t, opened)
}

func TestAnAnswerTheRequestRefusesIsRefusedByItsPath(t *testing.T) {
	opened := false
	greet := greet(func(context.Context, string) (useCase, error) { opened = true; return useCase{}, nil })

	got := greet.Execute(t.Context(), Call{Dir: "/repo", Answers: []domain.Answered{{Path: "nmae", Values: []string{"x"}}}})

	assert.Equal(t, http.StatusBadRequest, got.Status)
	require.NotNil(t, got.Envelope.Err)
	require.Len(t, got.Envelope.Err.Fields, 1)
	assert.Equal(t, "nmae", got.Envelope.Err.Fields[0].Path)
	assert.Equal(t, domain.CodeUnknown, got.Envelope.Err.Fields[0].Code)
	assert.False(t, opened)
}

func TestTheQuestionsAreAskedOfTheRequestAnsweredOnTheCallsDirectory(t *testing.T) {
	greet := greet(func(_ context.Context, dir string) (useCase, error) { return useCase{dir: dir}, nil })

	asked, err := greet.Questions(t.Context(), Call{Dir: "/a", Answers: named("x")})

	require.NoError(t, err)
	assert.Equal(t, "/a:x", asked.Fields[0].Spec().Label)
	assert.Empty(t, greet.Form.Fields[0].Spec().Label, "the catalogue lists the form of zero questions")
}

func TestTheQuestionsOfAnInvalidCallAreRefusedBeforeAnythingOpens(t *testing.T) {
	opened := false
	greet := greet(func(context.Context, string) (useCase, error) { opened = true; return useCase{}, nil })

	_, relative := greet.Questions(t.Context(), Call{Dir: "repo"})
	_, unknown := greet.Questions(t.Context(), Call{Dir: "/repo", Answers: []domain.Answered{{Path: "nmae", Values: []string{"x"}}}})

	assert.Equal(t, domain.KindInvalid, domain.AsError(domain.AsErrorParams{Err: relative}).Kind)
	assert.Equal(t, domain.KindInvalid, domain.AsError(domain.AsErrorParams{Err: unknown}).Kind)
	assert.False(t, opened)
}

func TestAStalePlanIsFollowedUpByItsRequestAsADryRun(t *testing.T) {
	stale := useCase{refusal: &domain.Error{Kind: domain.KindPrecondition, Code: domain.CodePlanStale}}
	greet := greet(func(context.Context, string) (useCase, error) { return stale, nil })

	got := greet.Execute(t.Context(), Call{Dir: "/repo", Answers: named("x")})

	assert.Equal(t, http.StatusPreconditionFailed, got.Status)
	assert.Equal(t, domain.Replan(got.Envelope.Err, "greet", named("x")), got.Envelope.FollowUps)
	assert.Len(t, got.Envelope.FollowUps, 1)
}

func TestACommandIsFoundByItsName(t *testing.T) {
	catalogue := Catalogue{{Name: "create"}, {Name: "extract"}}

	found, ok := catalogue.Find("extract")
	_, missing := catalogue.Find("clean")

	assert.True(t, ok)
	assert.Equal(t, "extract", found.Name)
	assert.False(t, missing)
}

func TestACallIsAskedAndRunFromItsDirectory(t *testing.T) {
	var asked, ran domain.Caller
	command := Bind("greet", func(context.Context, string) (useCase, error) { return useCase{}, nil },
		func(_ useCase, _ context.Context, req request) (questions, error) {
			asked = req.Caller
			return questions{}, nil
		},
		func(_ useCase, _ context.Context, req request, _ domain.RunOptions) (outcome, error) {
			ran = req.Caller
			return outcome{}, nil
		})

	_, err := command.Questions(t.Context(), Call{Dir: "/code/app/.trees/feat-a"})
	command.Execute(t.Context(), Call{Dir: "/code/app/.trees/feat-a"})

	require.NoError(t, err)
	want := domain.Caller{Dir: "/code/app/.trees/feat-a", Named: true, Cwd: "/code/app/.trees/feat-a"}
	assert.Equal(t, want, asked)
	assert.Equal(t, want, ran)
}
