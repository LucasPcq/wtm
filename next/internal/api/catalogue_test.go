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

		got := greet.Execute(t.Context(), Call{Dir: "/repo", Request: json.RawMessage(`{}`)})

		assert.Equal(t, status, got.Status, kind)
		assert.Equal(t, "greet.refused", got.Envelope.Err.Code, kind)
	}
}

func TestARequestIsDecodedStrictly(t *testing.T) {
	opened := false
	greet := Bind("greet", domain.Form{}, func(context.Context, string) (useCase, error) { opened = true; return useCase{}, nil }, useCase.Run)

	for request, message := range map[string]string{
		`{"nmae": "x"}`:               `invalid greet request: json: unknown field "nmae"`,
		`{"name": "x"}{"evil": 1}`:    "invalid greet request: unexpected data after the request",
		`{"name": "x"}garbage`:        "invalid greet request: unexpected data after the request",
		`{"name": "x"} {"name": "y"}`: "invalid greet request: unexpected data after the request",
	} {
		got := greet.Execute(t.Context(), Call{Dir: "/repo", Request: json.RawMessage(request)})

		assert.Equal(t, http.StatusBadRequest, got.Status, request)
		assert.Equal(t, message, got.Envelope.Err.Message, request)
	}
	assert.False(t, opened)
}

func TestADirectoryThatIsNotAbsoluteIsRefusedBeforeAnythingOpens(t *testing.T) {
	opened := false
	greet := Bind("greet", domain.Form{}, func(context.Context, string) (useCase, error) { opened = true; return useCase{}, nil }, useCase.Run)

	for _, dir := range []string{"", ".", "repo/main"} {
		got := greet.Execute(t.Context(), Call{Dir: dir, Request: json.RawMessage(`{"name": "x"}`)})

		assert.Equal(t, http.StatusBadRequest, got.Status, dir)
		assert.Equal(t, domain.CodeInvalidRequest, got.Envelope.Err.Code, dir)
	}
	assert.False(t, opened)
}

func TestACommandIsFoundByItsName(t *testing.T) {
	catalogue := Catalogue{{Name: "create"}, {Name: "extract"}}

	found, ok := catalogue.Find("extract")
	_, missing := catalogue.Find("clean")

	assert.True(t, ok)
	assert.Equal(t, "extract", found.Name)
	assert.False(t, missing)
}
