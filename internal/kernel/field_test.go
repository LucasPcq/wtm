package kernel_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/kernel"
)

func TestAFieldStateReadsBackFromItsJSON(t *testing.T) {
	states := []kernel.FieldState{
		{Path: "from", Status: kernel.FieldDefaulted, Origin: kernel.OriginConfig, Value: kernel.Text("main"), Remembered: kernel.Text("dev")},
		{Path: "push", Status: kernel.FieldProvided, Origin: kernel.OriginRequest, Value: kernel.Bool(true)},
		{Path: "branches", Status: kernel.FieldProvided, Origin: kernel.OriginRequest, Value: kernel.List{"a", "b"}},
		{Path: "decisions", Status: kernel.FieldDefaulted, Value: kernel.Decisions{"PORT": "keep"}},
		{Path: "message", Status: kernel.FieldSkipped, SkipReason: "test.not_needed"},
		{Path: "target", Status: kernel.FieldMissing, Errors: []kernel.FieldError{{Path: "target", Code: kernel.CodeRequired}}},
	}
	for _, state := range states {
		data, err := json.Marshal(state)
		require.NoError(t, err)
		var decoded kernel.FieldState
		require.NoError(t, json.Unmarshal(data, &decoded))
		assert.Equal(t, state, decoded, string(data))
	}
}

func TestAFieldStateWhoseValueIsNoValueIsRefused(t *testing.T) {
	var decoded kernel.FieldState
	assert.Error(t, json.Unmarshal([]byte(`{"path":"x","status":"provided","value":3}`), &decoded))
}
