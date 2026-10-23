package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAStreamSendsEachLineOfItsRequestWatchedFromTheCallsDirectory(t *testing.T) {
	w := &watched{lines: []string{`{"type":"snapshot"}`, `{"type":"ready"}`}}
	stream := BindStream[request, questions]("events", w.watch)
	var sent []string

	got := stream.Watch(t.Context(), Call{Dir: "/repo", Answers: []domain.Answered{{Path: "name", Values: []string{"x"}}}}, func(line json.RawMessage) error {
		sent = append(sent, string(line))
		return nil
	})

	assert.Equal(t, http.StatusOK, got.Status)
	assert.Equal(t, request{Name: "x", Caller: domain.Caller{Dir: "/repo", Named: true, Cwd: "/repo"}}, w.req)
	assert.Equal(t, w.lines, sent)
	assert.Equal(t, "events", stream.Name)
	assert.NotEmpty(t, stream.Form.Fields)
}

func TestAStreamRefusedRespondsWithTheStatusOfItsKind(t *testing.T) {
	refused := &watched{refusal: &domain.Error{Kind: domain.KindPrecondition, Code: "events.config_not_found"}}

	got := BindStream[request, questions]("events", refused.watch).Watch(t.Context(), Call{Dir: "/repo"}, func(json.RawMessage) error { return nil })

	assert.Equal(t, http.StatusPreconditionFailed, got.Status)
	require.NotNil(t, got.Envelope.Err)
	assert.Equal(t, "events.config_not_found", got.Envelope.Err.Code)
}

func TestAStreamRefusesARelativeDirectoryOrAnUnknownAnswerBeforeWatching(t *testing.T) {
	for name, call := range map[string]Call{
		"relative": {Dir: "repo"},
		"unknown":  {Dir: "/repo", Answers: []domain.Answered{{Path: "nope", Values: []string{"x"}}}},
	} {
		t.Run(name, func(t *testing.T) {
			w := &watched{}

			got := BindStream[request, questions]("events", w.watch).Watch(t.Context(), call, func(json.RawMessage) error { return nil })

			assert.Equal(t, http.StatusBadRequest, got.Status)
			assert.False(t, w.watched)
		})
	}
}

func TestAStreamOfACallNamingNoDirectoryStandsNowhere(t *testing.T) {
	w := &watched{}

	got := BindStream[request, questions]("events", w.watch).Watch(t.Context(), Call{}, func(json.RawMessage) error { return nil })

	assert.Equal(t, http.StatusOK, got.Status)
	assert.True(t, w.watched)
	assert.Equal(t, domain.Caller{}, w.req.Caller)
}
