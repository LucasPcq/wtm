package cli

import (
	"fmt"
	"strconv"

	"github.com/LucasPcq/wtm/next/internal/command/checkout"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// titleWidth is where a pull request's title is cut in the picker, so the authors line up.
const titleWidth = 36

// words are how the command line words each field of checkout's form.
func words(q checkout.Questions, field domain.Field) form.Words {
	path := field.Spec().Path
	newWorktree := form.NewWorktree{EnvHelp: "Where its .env files come from: config, example, main or parent; config.toml's strategy by default", Configured: q.Configured, Behind: q.Behind}
	if said, ok := newWorktree.Words(path); ok {
		// the source of create is the parent here: what is fast-forwarded is the pull request's own branch
		if path == domain.PathFastForward {
			said.Label, said.Help = "Branch update", "Fast-forward the pull request's branch to origin first when it exists locally and is behind"
		}
		return said
	}

	switch path {
	case checkout.PathPR:
		return form.Words{Flag: "<number>", Label: "Pull request", Help: "The open pull request whose branch to check out", Options: prRows(q.PR.Options)}
	case checkout.PathFrom:
		return form.Words{Flag: "--from", Label: "Parent branch", Help: "The parent wtm sync rebases it onto; the branch the pull request merges into by default"}
	case checkout.PathMine:
		return form.Words{Flag: "--mine", Label: "Mine", Help: "List only your pull requests"}
	case checkout.PathReview:
		return form.Words{Flag: "--review", Label: "Review requested", Help: "List only the pull requests waiting for your review"}
	case checkout.PathNoFetch:
		return form.Words{Flag: "--no-fetch", Label: "No fetch", Help: "Read the pull request's branch as last fetched"}
	}

	return form.Words{}
}

// prRows word each pull request of the picker: its number, its title cut so the authors line up, its author, and draft when it is one; the trail names it by its number.
func prRows(options []domain.Option) map[string]form.OptionWords {
	rows := make(map[string]form.OptionWords, len(options))
	for _, option := range options {
		pr := option.PR
		if pr == nil {
			continue
		}
		title := []rune(pr.Title)
		if len(title) > titleWidth {
			title = append(title[:titleWidth-1], '…')
		}
		number := "#" + strconv.Itoa(pr.Number)
		label := fmt.Sprintf("%-7s%-*s%s", number, titleWidth+2, string(title), pr.Author)
		if pr.Draft {
			label += " · draft"
		}
		rows[option.Value] = form.OptionWords{Label: label, Short: number}
	}

	return rows
}
