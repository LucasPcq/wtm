package cli

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/command/tree"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/charmbracelet/x/ansi"
)

const (
	connectorBranch = "├─ "
	connectorLast   = "└─ "
	gutterPipe      = "│  "
	gutterBlank     = "   "
	badgeGap        = "  "
	glyphAhead      = "↑"
	glyphBehind     = "↓"
)

// A tone is how a badge is coloured: whole, or only its glyph.
type tone int

const (
	toneMuted     tone = iota // the whole badge muted
	toneSuccess               // the whole badge green
	toneWarning               // the whole badge yellow
	toneDanger                // the whole badge red
	toneAttention             // a ! glyph, yellow, then its text
	toneFailure               // a ✗ glyph, red, then its text
)

// A badge is one thing said of a node, after its branch.
type badge struct {
	text string
	tone tone
}

// badgesOf are what a node's row says of it, in their order: no worktree, the jobs up by name, its pull request, its commits past its parent, where it stands against origin, its state, locked, needs sync, cycle.
func badgesOf(item tree.Item) []badge {
	d := item.Detail
	var badges []badge
	if d.Virtual {
		badges = append(badges, badge{"(no worktree)", toneMuted})
	}
	if len(d.Services) > 0 {
		badges = append(badges, badge{"services " + strings.Join(d.Services, ", "), toneSuccess})
	}
	if d.PR != nil {
		badges = append(badges, prBadge(*d.PR))
	}
	if d.Ahead != nil && *d.Ahead > 0 {
		badges = append(badges, badge{glyphAhead + strconv.Itoa(*d.Ahead), toneMuted})
	}
	if origin, ok := originBadge(d.Origin); ok {
		badges = append(badges, origin)
	}
	switch {
	case item.Status == domain.StatusFailed:
		badges = append(badges, badge{"unreadable", toneFailure})
	case d.Missing:
		badges = append(badges, badge{"missing", toneFailure})
	case d.Rebasing:
		badges = append(badges, badge{"rebasing", toneAttention})
	case d.Dirty:
		badges = append(badges, badge{"dirty", toneAttention})
	}
	if d.Locked {
		badges = append(badges, badge{"locked", toneAttention})
	}
	if d.NeedsSync {
		badges = append(badges, badge{"needs sync", toneAttention})
	}
	if d.Cycle {
		badges = append(badges, badge{"cycle", toneAttention})
	}

	return badges
}

func prBadge(pr tree.PR) badge {
	label := "PR #" + strconv.Itoa(pr.Number)
	if pr.State == worktree.PullRequestOpen {
		return badge{label, toneSuccess}
	}
	return badge{label + " " + string(pr.State), toneMuted}
}

// originBadge is where the branch stands against origin, none when up to date or not on origin.
func originBadge(origin *worktree.Origin) (badge, bool) {
	if origin == nil {
		return badge{}, false
	}

	ahead, behind := glyphAhead+strconv.Itoa(origin.Ahead), glyphBehind+strconv.Itoa(origin.Behind)
	switch origin.State {
	case worktree.Ahead:
		return badge{"origin " + ahead, toneMuted}, true
	case worktree.Behind:
		return badge{"origin " + behind, toneWarning}, true
	case worktree.Diverged:
		return badge{"origin " + ahead + " " + behind, toneDanger}, true
	case worktree.NoOrigin, worktree.UpToDate:
	}
	return badge{}, false
}

// plain is the badge as text, its glyph before it.
func (b badge) plain() string {
	switch b.tone {
	case toneAttention:
		return output.GlyphAttention + " " + b.text
	case toneFailure:
		return output.GlyphFailure + " " + b.text
	case toneMuted, toneSuccess, toneWarning, toneDanger:
	}
	return b.text
}

func (b badge) styled(st *output.Styles) string {
	switch b.tone {
	case toneMuted:
		return st.Muted.Render(b.text)
	case toneSuccess:
		return st.Success.Render(b.text)
	case toneWarning:
		return st.Warning.Render(b.text)
	case toneDanger:
		return st.Danger.Render(b.text)
	case toneAttention:
		return st.Warning.Render(output.GlyphAttention) + " " + b.text
	case toneFailure:
		return st.Danger.Render(output.GlyphFailure) + " " + b.text
	}
	return b.text
}

// A row is one node as the text draws it: its connectors, its branch, its badges.
type row struct {
	prefix string
	item   tree.Item
}

// rowsOf are the nodes of read depth first, each with the connectors its place in the tree gives it.
func rowsOf(read tree.Outcome) []row {
	bySubject := map[string]tree.Item{}
	for _, item := range read.Items {
		bySubject[item.Subject] = item
	}

	var rows []row
	var walk func(item tree.Item, gutter, connector string)
	walk = func(item tree.Item, gutter, connector string) {
		rows = append(rows, row{prefix: gutter + connector, item: item})
		switch connector {
		case connectorBranch:
			gutter += gutterPipe
		case connectorLast:
			gutter += gutterBlank
		}
		for i, child := range item.Detail.Children {
			next := connectorBranch
			if i == len(item.Detail.Children)-1 {
				next = connectorLast
			}
			walk(bySubject[child], gutter, next)
		}
	}
	for _, item := range read.Items {
		if item.Detail.Parent == nil {
			walk(item, "", "")
		}
	}

	return rows
}

// drawTree draws the forest read, a row per node with its badges aligned after the widest branch, then each worktree that could not be read and the warnings.
func drawTree(w io.Writer, read tree.Outcome, _ output.View) {
	st := output.StylesOf(w)
	rows := rowsOf(read)
	if len(rows) == 0 {
		output.UnchangedLine(w, "no worktree on a branch")
	}

	width := 0
	for _, r := range rows {
		width = max(width, ansi.StringWidth(r.prefix+r.item.Subject))
	}
	for _, r := range rows {
		branch := r.item.Subject
		if r.item.Detail.Virtual {
			branch = st.Muted.Render(branch)
		}
		line := st.Muted.Render(r.prefix) + branch
		var badges []string
		for _, b := range badgesOf(r.item) {
			badges = append(badges, b.styled(st))
		}
		if len(badges) > 0 {
			line += strings.Repeat(" ", width-ansi.StringWidth(r.prefix+r.item.Subject)) + badgeGap + strings.Join(badges, badgeGap)
		}
		fmt.Fprintln(w, output.Indent+line)
	}

	for _, item := range read.Items {
		if item.Err != nil {
			output.ErrorLine(w, item.Err.Error())
		}
	}
	for _, warning := range read.Warnings {
		output.WarningLine(w, warning.Message)
	}
}
