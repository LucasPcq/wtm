package output

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestARememberedValueCarriesItsNote(t *testing.T) {
	assert.Equal(t, WillBeRemembered, RememberedNote([]string{"isolation", "env_from"}, "env_from", WillBeRemembered))
	assert.Empty(t, RememberedNote([]string{"isolation"}, "env_from", WillBeRemembered))
}

func TestTheNoteOfARowFollowsItsLastValue(t *testing.T) {
	cases := map[string]struct {
		params NotedParams
		want   []string
	}{
		"values": {NotedParams{Values: []string{"main", "topic"}, Note: Remembered, None: "none"}, []string{"main", "topic · remembered"}},
		"none":   {NotedParams{Note: Remembered, None: "keep as-is"}, []string{"keep as-is · remembered"}},
		"noNote": {NotedParams{Values: []string{"main"}, None: "none"}, []string{"main"}},
		"empty":  {NotedParams{None: "none"}, nil},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, c.want, Noted(c.params))
		})
	}
}
