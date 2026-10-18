package list_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/list"
	"github.com/stretchr/testify/assert"
)

func TestAWorktreeOffersWhatItsStateAllows(t *testing.T) {
	cases := map[string]struct {
		detail list.Detail
		offers []list.Action
	}{
		"linked":   {detail: list.Detail{Branch: "feat/a"}, offers: []list.Action{list.ActionGo, list.ActionRunUp, list.ActionRunDown, list.ActionLogs, list.ActionClean}},
		"with_pr":  {detail: list.Detail{Branch: "feat/a", PR: &list.PR{Number: 12}}, offers: []list.Action{list.ActionGo, list.ActionOpenPR, list.ActionRunUp, list.ActionRunDown, list.ActionLogs, list.ActionClean}},
		"main":     {detail: list.Detail{Branch: "main", Main: true}, offers: []list.Action{list.ActionGo, list.ActionRunUp, list.ActionRunDown, list.ActionLogs}},
		"detached": {detail: list.Detail{Path: "/repo/.trees/x"}, offers: []list.Action{list.ActionGo}},
		"missing":  {detail: list.Detail{Branch: "feat/gone", Missing: true}, offers: []list.Action{list.ActionClean}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, c.offers, list.Offers(c.detail))
		})
	}
}

func TestTheActiveWorktreeIsTheDeepestHoldingTheDirectory(t *testing.T) {
	items := []list.Item{
		{Subject: "main", Detail: &list.Detail{Path: "/repo/main"}},
		{Subject: "feat/a", Detail: &list.Detail{Path: "/repo/main/.trees/a"}},
		{Subject: "feat/ab", Detail: &list.Detail{Path: "/repo/main/.trees/ab"}},
		{Subject: "broken"},
	}

	assert.Equal(t, "feat/a", list.Holding(items, "/repo/main/.trees/a/src"))
	assert.Equal(t, "main", list.Holding(items, "/repo/main"))
	assert.Empty(t, list.Holding(items, "/elsewhere"))
}
