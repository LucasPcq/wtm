package form

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnteringAValueNothingDependsOnDoesNoIO(t *testing.T) {
	r := &repo{}
	d := newWorld(t, r, worldParams{Request: `{"branches":["feat/a"],"from":"main"}`}).open(t)
	before := r.io()

	d.key("down", "enter")
	d.key("esc")
	d.key("enter")

	assert.Equal(t, before, r.io(), "the facts and the choices already read serve every value typed")
	assert.Contains(t, d.trail(), "Isolation: verbatim")
}

func TestChoicesAreListedAgainOnlyWhenWhatTheyDependOnChanges(t *testing.T) {
	r := &repo{}
	d := newWorld(t, r, worldParams{}).open(t)
	d.typed("feat/a")
	d.key("enter")
	require.Equal(t, 1, r.choices["from"])

	d.key("esc", "enter")
	assert.Equal(t, 1, r.choices["from"], "the same branches give the same choices")

	d.key("esc")
	d.typed("feat/b")
	d.key("enter")
	assert.Equal(t, 2, r.choices["from"])
}

func TestSlowChoicesShowTheFieldUntilTheyArrive(t *testing.T) {
	d := newWorld(t, &repo{existing: []string{"feat/old"}}, worldParams{}).open(t)
	d.typed("feat/a")

	started := answers(d.update(keyMsg("enter")))
	require.Len(t, started, 1)
	loading := d.update(started[0])

	assert.Contains(t, d.view(), choicesText)
	assert.NotContains(t, d.view(), "feat/old")
	d.run(loading)
	assert.NotContains(t, d.view(), choicesText)
	assert.Contains(t, d.view(), "feat/old")
}

func TestRefreshObservesAgainAndListsWhatIsNew(t *testing.T) {
	r := &repo{}
	d := newWorld(t, r, worldParams{Request: `{"branches":["feat/a"]}`}).open(t)
	r.changed("feat/new")

	d.key("r")

	assert.Equal(t, 2, r.observes)
	assert.Equal(t, 2, r.choices["from"])
	assert.Contains(t, d.view(), "feat/new")
}
