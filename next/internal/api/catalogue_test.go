package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestEachCallOpensTheUseCaseOnItsDirectory(t *testing.T) {
	greet := Bind("greet", domain.Form{}, func(_ context.Context, dir string) (useCase, error) { return useCase{dir: dir}, nil }, useCase.Run)

	first := greet.Execute(t.Context(), Call{Dir: "/a", Request: json.RawMessage(`{"name": "x"}`)})
	second := greet.Execute(t.Context(), Call{Dir: "/b", Request: json.RawMessage(`{"name": "y"}`), Options: domain.RunOptions{Surface: "desktop"}})

	assert.Equal(t, Response{Status: http.StatusOK, Envelope: domain.Envelope{Command: "greet", Op: "/a:x:api"}}, first)
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
		greet := Bind("greet", domain.Form{}, func(context.Context, string) (useCase, error) { return refusing, nil }, useCase.Run)

		got := greet.Execute(t.Context(), Call{Request: json.RawMessage(`{}`)})

		assert.Equal(t, status, got.Status, kind)
		assert.Equal(t, "greet.refused", got.Envelope.Err.Code, kind)
	}
}

func TestARequestIsDecodedStrictly(t *testing.T) {
	greet := Bind("greet", domain.Form{}, func(context.Context, string) (useCase, error) { return useCase{}, nil }, useCase.Run)

	got := greet.Execute(t.Context(), Call{Request: json.RawMessage(`{"nmae": "x"}`)})

	assert.Equal(t, http.StatusBadRequest, got.Status)
	assert.Equal(t, `invalid greet request: json: unknown field "nmae"`, got.Envelope.Err.Message)
}

func TestACommandIsFoundByItsName(t *testing.T) {
	catalogue := Catalogue{{Name: "create"}, {Name: "extract"}}

	found, ok := catalogue.Find("extract")
	_, missing := catalogue.Find("clean")

	assert.True(t, ok)
	assert.Equal(t, "extract", found.Name)
	assert.False(t, missing)
}
