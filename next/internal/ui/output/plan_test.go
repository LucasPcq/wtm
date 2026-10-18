package output

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAnAnswerIsMarkedRememberedOverRecalled(t *testing.T) {
	marks := Marks{Recalled: []string{"isolation", "env_from"}, Remembered: []string{"env_from"}, Mark: WillBeRemembered}

	assert.Equal(t, WillBeRemembered, marks.Of("env_from"))
	assert.Equal(t, Recalled, marks.Of("isolation"))
	assert.Empty(t, marks.Of("fast_forward"))
}

func TestTheNoteOfARowFollowsItsLastValue(t *testing.T) {
	cases := map[string]struct {
		params NotedParams
		want   []string
	}{
		"values":  {NotedParams{Values: []string{"main", "topic"}, Note: Saved, None: "none"}, []string{"main", "topic · saved for next runs"}},
		"none":    {NotedParams{Note: Saved, None: "keep as-is"}, []string{"keep as-is · saved for next runs"}},
		"no_note": {NotedParams{Values: []string{"main"}, None: "none"}, []string{"main"}},
		"empty":   {NotedParams{None: "none"}, nil},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, c.want, Noted(c.params))
		})
	}
}

func TestAMarkedEnvThatFollowsTheConfigReadsAsTheConfigDefault(t *testing.T) {
	assert.Equal(t, "config default (example) · will be remembered", EnvValue(EnvValueParams{Strategy: "example", FollowsConfig: true, Mark: WillBeRemembered}))
	assert.Equal(t, "example · will be remembered", EnvValue(EnvValueParams{Strategy: "example", Mark: WillBeRemembered}))
	assert.Equal(t, "example", EnvValue(EnvValueParams{Strategy: "example", FollowsConfig: true}))
}
