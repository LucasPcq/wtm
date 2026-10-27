package worktree

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAWorktreeOffersEachActionOrSaysWhyNot(t *testing.T) {
	cases := map[string]struct {
		Params OfferParams
		Want   map[Action]Unoffered
	}{
		"with_pr": {
			OfferParams{PullRequest: true, OnOrigin: true, Record: RecordRead},
			map[Action]Unoffered{},
		},
		"no_pr": {
			OfferParams{OnOrigin: true, Record: RecordRead},
			map[Action]Unoffered{ActionOpenPR: UnofferedNoPullRequest},
		},
		"not_on_origin": {
			OfferParams{PullRequest: true, Record: RecordRead},
			map[Action]Unoffered{ActionFastForward: UnofferedNotOnOrigin},
		},
		"main": {
			OfferParams{Main: true, PullRequest: true, OnOrigin: true},
			map[Action]Unoffered{ActionSync: UnofferedMainWorktree, ActionReparent: UnofferedMainWorktree, ActionClean: UnofferedMainWorktree},
		},
		"base_branch": {
			OfferParams{Base: true, PullRequest: true, OnOrigin: true},
			map[Action]Unoffered{ActionReparent: UnofferedBaseBranch, ActionClean: UnofferedBaseBranch},
		},
		"missing": {
			OfferParams{Missing: true, PullRequest: true, OnOrigin: true},
			map[Action]Unoffered{ActionGo: UnofferedMissing, ActionOpenPR: UnofferedMissing, ActionRunUp: UnofferedMissing, ActionRunDown: UnofferedMissing, ActionLogs: UnofferedMissing, ActionFastForward: UnofferedMissing, ActionSync: UnofferedMissing, ActionReparent: UnofferedMissing},
		},
		"detached": {
			OfferParams{Detached: true},
			map[Action]Unoffered{ActionOpenPR: UnofferedNoBranch, ActionRunUp: UnofferedNoBranch, ActionRunDown: UnofferedNoBranch, ActionLogs: UnofferedNoBranch, ActionFastForward: UnofferedNoBranch, ActionSync: UnofferedNoBranch, ActionReparent: UnofferedNoBranch},
		},
		"meta_unreadable": {
			OfferParams{PullRequest: true, OnOrigin: true, Record: RecordUnreadable},
			map[Action]Unoffered{ActionReparent: UnofferedUnreadable},
		},
		"unrecorded": {
			OfferParams{PullRequest: true, OnOrigin: true},
			map[Action]Unoffered{ActionReparent: UnofferedUnrecorded},
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			for _, action := range Actions() {
				assert.Equal(t, c.Want[action], Unoffer(c.Params, action), action)
			}
		})
	}
}
