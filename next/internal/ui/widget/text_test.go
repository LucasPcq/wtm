package widget

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTextChecksItsValueOffTheLoopBeforeAnswering(t *testing.T) {
	var checked []string
	x := NewText("feat/a", func(value string) error { checked = append(checked, value); return nil })

	_, cmd := x.Update(keyOf("enter"))
	require.NotNil(t, cmd)
	assert.Empty(t, checked)

	assert.Equal(t, Done, textSettled(x, cmd))
	assert.Equal(t, []string{"feat/a"}, checked)
}

func TestTextShowsARefusalUntilTheValueChanges(t *testing.T) {
	x := NewText("bad", refusingBad)

	_, cmd := x.Update(keyOf("enter"))

	assert.Equal(t, Stay, textSettled(x, cmd))
	assert.Contains(t, plain(x.View(Size{Width: 40}, plainStyles())), "✗ Not a valid name")
	x.Update(keyOf("backspace"))
	assert.NotContains(t, plain(x.View(Size{Width: 40}, plainStyles())), "Not a valid name")
	verdict, _ := x.Update(keyOf("esc"))
	assert.Equal(t, Back, verdict)
}

func TestTextKeepsTheKeysTypedWhileItChecks(t *testing.T) {
	x := NewText("bad", refusingBad)

	_, cmd := x.Update(keyOf("enter"))
	x.Update(keyOf("backspace"))
	x.Update(keyOf("backspace"))
	x.Update(keyOf("backspace"))

	assert.Equal(t, Stay, textSettled(x, cmd))
	assert.Empty(t, x.Value())
}

func TestAWidgetShowsItsErrorAsWritten(t *testing.T) {
	x := NewText("bad", func(string) error { return errors.New("feat/other already has a worktree") })

	_, cmd := x.Update(keyOf("enter"))
	textSettled(x, cmd)

	assert.Contains(t, plain(x.View(Size{Width: 60}, plainStyles())), "✗ feat/other already has a worktree")
}
