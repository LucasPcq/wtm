package render_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LucasPcq/wtm/internal/styles"
	"github.com/LucasPcq/wtm/internal/surface/cli/render"
)

func TestTheGenericPlanIsItsFieldsInTheirOwnOrder(t *testing.T) {
	rows := render.GenericDetail{}.Plan(json.RawMessage(`{"isolation":"isolated","branches":["feat/a","feat/b"],"count":2,"nested":{"a":1}}`))

	assert.Equal(t, []styles.PlanRow{
		{Label: "Isolation", Values: []string{"isolated"}},
		{Label: "Branches", Values: []string{"feat/a", "feat/b"}},
		{Label: "Count", Values: []string{"2"}},
		{Label: "Nested", Values: []string{`{"a":1}`}},
	}, rows)
}

func TestAPlanThatIsNotAnObjectIsOneRowWithoutALabel(t *testing.T) {
	rows := render.GenericDetail{}.Plan(json.RawMessage(`"feat/a"`))

	assert.Equal(t, []styles.PlanRow{{Values: []string{"feat/a"}}}, rows)
}

func TestAGenericPlanSpellsEachKeyAsALabelAndEachValueAsText(t *testing.T) {
	rows := render.GenericDetail{}.Plan(json.RawMessage(`{"branches":["feat/a","feat/b"],"base_branch":"main","pushAfter":true,"hooks":[],"env":{".env":"link",".env.local":"copy"},"ports":3,"note":null}`))

	assert.Equal(t, []styles.PlanRow{
		{Label: "Branches", Values: []string{"feat/a", "feat/b"}},
		{Label: "Base branch", Values: []string{"main"}},
		{Label: "Push after", Values: []string{"yes"}},
		{Label: "Hooks", Values: []string{"none"}},
		{Label: "Env", Values: []string{".env → link", ".env.local → copy"}},
		{Label: "Ports", Values: []string{"3"}},
		{Label: "Note", Values: []string{"none"}},
	}, rows)
}
