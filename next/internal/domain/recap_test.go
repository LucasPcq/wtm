package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAnAnswerIsMarkedRememberedOverRecalled(t *testing.T) {
	marks := Marks{Recalled: []string{"isolation", "env_from"}, Remembered: []string{"env_from"}, Mark: MarkRemember}

	assert.Equal(t, MarkRemember, marks.Of("env_from"))
	assert.Equal(t, MarkRecalled, marks.Of("isolation"))
	assert.Empty(t, marks.Of("fast_forward"))
}
