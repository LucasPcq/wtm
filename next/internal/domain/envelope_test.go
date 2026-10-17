package domain

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnEnvelopeWithoutASpellingNamesEachFieldByItsPath(t *testing.T) {
	refused := Invalid([]FieldError{{Path: "branches[1]", Code: "required", Template: "{path} is required"}})

	data, err := json.Marshal(Envelope{Command: "create", Err: refused})

	require.NoError(t, err)
	assert.JSONEq(t, `{"v": 2, "command": "create", "ok": false, "follow_ups": [], "warnings": [],
		"error": {"kind": "invalid", "code": "request.invalid", "message": "the request is invalid",
			"fields": [{"path": "branches[1]", "code": "required", "flag": "branches[1]", "message": "branches[1] is required"}]}}`, string(data))
}

func TestASpellingNamesEachFieldAsTheSurfaceDoes(t *testing.T) {
	refused := Invalid([]FieldError{{Path: "branches[1]", Code: "required", Template: "{path} is required"}})

	data, err := Envelope{Command: "create", Err: refused}.JSON(func(path string) string { return "<" + path + ">" })

	require.NoError(t, err)
	assert.Contains(t, string(data), `"flag":"<branches[1]>","message":"<branches[1]> is required"`)
}

func TestAnErrorIsEncodedAsTheEnvelopesErrorWithItsCauseInItsText(t *testing.T) {
	failed := AsError(AsErrorParams{Err: assert.AnError})

	data, err := json.Marshal(failed)

	require.NoError(t, err)
	assert.JSONEq(t, `{"kind": "internal", "code": "internal", "message": "internal error: `+assert.AnError.Error()+`"}`, string(data))
}

func TestAnItemWithoutDetailHasAnEmptyOne(t *testing.T) {
	data, err := json.Marshal(Envelope{Command: "create", Items: []Item{{Subject: "feat/x", Status: StatusSkipped}}})

	require.NoError(t, err)
	assert.Contains(t, string(data), `"items":[{"subject":"feat/x","status":"skipped","detail":{}}]`)
}

func TestANilDetailIsNoDetail(t *testing.T) {
	var none *struct{ Path string }

	assert.Nil(t, DetailOf(none))
	assert.Equal(t, &struct{ Path string }{"/p"}, DetailOf(&struct{ Path string }{"/p"}))
}
