package list_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/list"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
	"github.com/stretchr/testify/assert"
)

func TestAWorktreeOffersWhatItsStateAllows(t *testing.T) {
	cases := map[string]struct {
		detail list.Detail
		offers []worktree.Action
	}{
		"linked":           {detail: list.Detail{Branch: "feat/a"}, offers: []worktree.Action{worktree.ActionGo, worktree.ActionExec, worktree.ActionRunUp, worktree.ActionRunDown, worktree.ActionLogs, worktree.ActionSync, worktree.ActionClean}},
		"on_origin":        {detail: list.Detail{Branch: "feat/a", Origin: &worktree.Origin{State: worktree.UpToDate}}, offers: []worktree.Action{worktree.ActionGo, worktree.ActionExec, worktree.ActionRunUp, worktree.ActionRunDown, worktree.ActionLogs, worktree.ActionFastForward, worktree.ActionSync, worktree.ActionClean}},
		"main_on_origin":   {detail: list.Detail{Branch: "main", Main: true, Origin: &worktree.Origin{State: worktree.Behind, Behind: 1}}, offers: []worktree.Action{worktree.ActionGo, worktree.ActionExec, worktree.ActionRunUp, worktree.ActionRunDown, worktree.ActionLogs, worktree.ActionFastForward}},
		"with_pr":          {detail: list.Detail{Branch: "feat/a", PR: &list.PR{Number: 12, URL: "https://github.com/acme/acme/pull/12"}, Origin: &worktree.Origin{State: worktree.UpToDate}}, offers: []worktree.Action{worktree.ActionGo, worktree.ActionOpenPR, worktree.ActionExec, worktree.ActionRunUp, worktree.ActionRunDown, worktree.ActionLogs, worktree.ActionFastForward, worktree.ActionSync, worktree.ActionClean}},
		"main":             {detail: list.Detail{Branch: "main", Main: true}, offers: []worktree.Action{worktree.ActionGo, worktree.ActionExec, worktree.ActionRunUp, worktree.ActionRunDown, worktree.ActionLogs}},
		"detached":         {detail: list.Detail{Path: "/repo/.trees/x"}, offers: []worktree.Action{worktree.ActionGo, worktree.ActionExec, worktree.ActionClean}},
		"missing":          {detail: list.Detail{Branch: "feat/gone", Missing: true}, offers: []worktree.Action{worktree.ActionClean}},
		"missing_detached": {detail: list.Detail{Path: "/repo/.trees/x", Missing: true}, offers: []worktree.Action{worktree.ActionClean}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			var offered []worktree.Action
			for _, offer := range list.Actions(c.detail) {
				if offer.FollowUp != nil {
					offered = append(offered, offer.Action)
				}
			}
			assert.Equal(t, c.offers, offered)
			assert.Len(t, repository.Offered(list.Actions(c.detail)), len(c.offers))
		})
	}
}
