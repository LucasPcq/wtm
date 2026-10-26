package list_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/list"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/stretchr/testify/assert"
)

func TestAWorktreeOffersWhatItsStateAllows(t *testing.T) {
	cases := map[string]struct {
		detail list.Detail
		offers []list.Action
	}{
		"linked":           {detail: list.Detail{Branch: "feat/a"}, offers: []list.Action{list.ActionGo, list.ActionRunUp, list.ActionRunDown, list.ActionLogs, list.ActionClean}},
		"with_pr":          {detail: list.Detail{Branch: "feat/a", PR: &list.PR{Number: 12}}, offers: []list.Action{list.ActionGo, list.ActionOpenPR, list.ActionRunUp, list.ActionRunDown, list.ActionLogs, list.ActionClean}},
		"main":             {detail: list.Detail{Branch: "main", Main: true}, offers: []list.Action{list.ActionGo, list.ActionRunUp, list.ActionRunDown, list.ActionLogs}},
		"on_origin":        {detail: list.Detail{Branch: "feat/a", Origin: &worktree.Origin{State: worktree.UpToDate}}, offers: []list.Action{list.ActionGo, list.ActionRunUp, list.ActionRunDown, list.ActionLogs, list.ActionFastForward, list.ActionClean}},
		"main_on_origin":   {detail: list.Detail{Branch: "main", Main: true, Origin: &worktree.Origin{State: worktree.Behind, Behind: 1}}, offers: []list.Action{list.ActionGo, list.ActionRunUp, list.ActionRunDown, list.ActionLogs, list.ActionFastForward}},
		"detached":         {detail: list.Detail{Path: "/repo/.trees/x"}, offers: []list.Action{list.ActionGo}},
		"missing":          {detail: list.Detail{Branch: "feat/gone", Missing: true}, offers: []list.Action{list.ActionClean}},
		"missing_detached": {detail: list.Detail{Path: "/repo/.trees/x", Missing: true}, offers: []list.Action{list.ActionClean}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, c.offers, list.Offers(c.detail))
		})
	}
}
