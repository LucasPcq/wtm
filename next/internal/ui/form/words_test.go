package form_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/widget"
	"github.com/stretchr/testify/assert"
)

func TestEveryOptionTagHasItsWords(t *testing.T) {
	for _, tag := range domain.OptionTags() {
		t.Run(string(tag), func(t *testing.T) {
			assert.NotEmpty(t, form.Options([]domain.Option{{Value: "x", Tag: tag}}, form.Words{})[0].Badge)
		})
	}
}

func TestAnOptionReadsAsItsFactsUnlessItsWordsSayOtherwise(t *testing.T) {
	cases := map[string]struct {
		option domain.Option
		words  form.Words
		want   widget.Option
	}{
		"plain":       {option: domain.Option{Value: "feat/x"}, want: widget.Option{Label: "feat/x", Value: "feat/x"}},
		"worded":      {option: domain.Option{Value: "abort"}, words: form.Words{Options: map[string]form.OptionWords{"abort": {Label: "Abort"}}}, want: widget.Option{Label: "Abort", Value: "abort"}},
		"local":       {option: domain.Option{Value: "feat/x", Tag: domain.TagLocal}, want: widget.Option{Label: "feat/x", Value: "feat/x", Badge: "existing"}},
		"checked_out": {option: domain.Option{Value: "feat/x", Tag: domain.TagCheckedOut}, want: widget.Option{Label: "feat/x", Value: "feat/x", Badge: "worktree exists"}},
		"remote":      {option: domain.Option{Value: "feat/x", Tag: domain.TagRemote}, want: widget.Option{Label: "feat/x", Value: "feat/x", Badge: "on origin"}},
		"base_behind": {option: domain.Option{Value: "main", Base: true, Origin: &worktree.Origin{Behind: 2, State: worktree.Behind}}, want: widget.Option{Label: "main (default)", Value: "main", Badge: "2 behind"}},
		"ahead":       {option: domain.Option{Value: "dev", Origin: &worktree.Origin{Ahead: 1, State: worktree.Ahead}}, want: widget.Option{Label: "dev", Value: "dev", Badge: "1 ahead"}},
		"diverged":    {option: domain.Option{Value: "dev", Origin: &worktree.Origin{Ahead: 1, Behind: 3, State: worktree.Diverged}}, want: widget.Option{Label: "dev", Value: "dev", Badge: "1 ahead, 3 behind"}},
		"up_to_date":  {option: domain.Option{Value: "dev", Origin: &worktree.Origin{State: worktree.UpToDate}}, want: widget.Option{Label: "dev", Value: "dev"}},
		"current":     {option: domain.Option{Value: "/repo/x", Detached: true, Tag: domain.TagCurrent}, want: widget.Option{Label: "/repo/x (detached)", Value: "/repo/x", Badge: "current"}},
		"one_change":  {option: domain.Option{Value: "feat/x", Changes: 1}, want: widget.Option{Label: "feat/x", Value: "feat/x", Badge: "1 change"}},
		"changes":     {option: domain.Option{Value: "feat/x", Changes: 3}, want: widget.Option{Label: "feat/x", Value: "feat/x", Badge: "3 changes"}},
		"renamed":     {option: domain.Option{Value: "b.txt", Change: &worktree.Change{Path: "b.txt", OrigPath: "a.txt", Status: worktree.ChangeRenamed}}, want: widget.Option{Label: "a.txt → b.txt", Value: "b.txt", Badge: "ren"}},
		"untracked":   {option: domain.Option{Value: "n.txt", Change: &worktree.Change{Path: "n.txt", Status: worktree.ChangeUntracked}}, want: widget.Option{Label: "n.txt", Value: "n.txt", Badge: "new"}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, []widget.Option{c.want}, form.Options([]domain.Option{c.option}, c.words))
		})
	}
}

func TestTheNewWorktreeQuestionsAreWordedOnTheirData(t *testing.T) {
	one := form.NewWorktree{EnvHelp: "Where from", Configured: project.EnvMain, Behind: []string{"main", "topic"}}
	several := form.NewWorktree{Many: true}

	env, _ := one.Words(domain.PathEnvFrom)
	update, _ := one.Words(domain.PathFastForward)
	alone, _ := one.Words(domain.PathIsolation)
	each, _ := several.Words(domain.PathIsolation)
	unbehind, _ := several.Words(domain.PathFastForward)
	_, other := one.Words("from")

	assert.Equal(t, form.OptionWords{Label: "config default (main)", Short: "config default"}, env.Options["config"])
	assert.Equal(t, "Where from", env.Help)
	assert.Equal(t, form.OptionWords{Label: "Fast-forward main, topic to origin", Short: "fast-forward to origin"}, update.Options["true"])
	assert.Equal(t, form.OptionWords{Label: "Keep it as-is", Short: "keep as-is"}, update.Options["false"])
	assert.Equal(t, "Fast-forward to origin", unbehind.Options["true"].Label)
	assert.Equal(t, "isolated — its own ports and namespaces", alone.Options["isolated"].Label)
	assert.Equal(t, "isolated — each its own ports and namespaces", each.Options["isolated"].Label)
	assert.False(t, other)
}
