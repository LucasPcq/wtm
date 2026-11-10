package cli

import (
	"cmp"
	"io"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/command/envshow"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

const (
	none     = "—"
	withheld = "••••••"
)

// drawShow draws the worktree read: its keys to check, then its files app by app, each key with what it follows and its values; then the warnings.
func drawShow(w io.Writer, read envshow.Outcome, _ envshow.Request, _ output.View) {
	if len(read.Items) == 1 && read.Items[0].Detail != nil {
		drawDetail(w, *read.Items[0].Detail)
	}
	output.ReadEnd(w, read)
}

func drawDetail(w io.Writer, d envshow.Detail) {
	if len(d.Checks) == 0 {
		output.UnchangedLine(w, d.Worktree+" — nothing to check")
	} else {
		output.WarningLine(w, d.Worktree+" — "+output.Counted(len(d.Checks), "key to check", "keys to check"))
		rows := make([][]string, 0, len(d.Checks))
		for _, check := range d.Checks {
			rows = append(rows, []string{output.Indent + check.Key, check.File, checkWords(check)})
		}
		output.Columns(w, rows)
	}

	st := output.StylesOf(w)
	for _, app := range d.Apps {
		output.Blank(w)
		output.Message(w, app.Dir+st.Muted.Render("  "+appWords(app)))
		for _, file := range app.Files {
			output.Message(w, output.Indent+file.Name+st.Muted.Render("  "+originWords(file)))
			if len(file.Keys) == 0 {
				continue
			}
			rows := [][]string{{st.Muted.Render("KEY"), st.Muted.Render("FOLLOWS"), st.Muted.Render("IN MAIN"), st.Muted.Render("HERE")}}
			for _, key := range file.Keys {
				rows = append(rows, []string{key.Key, followsWords(key), value(key.Main, key.Withheld), hereValue(key)})
			}
			output.Columns(w, indented(rows))
		}
	}
}

func checkWords(check envshow.Check) string {
	if check.Kind == envshow.CheckUnlinked {
		if check.Job == "" {
			return "follows nothing: holds an address of this machine"
		}
		return "follows nothing: looks like " + check.Job + "'s " + check.Port
	}
	if check.Here != nil && check.Writes != nil {
		return "out of step: " + *check.Here + " here, wtm writes " + *check.Writes
	}
	return "out of step with what wtm writes"
}

func appWords(app envshow.App) string {
	said := []string{strconv.Itoa(app.Following) + " of " + output.Counted(app.Keys, "key", "keys") + " follow a job"}
	if len(app.Jobs) > 0 {
		verb := " run here"
		if len(app.Jobs) == 1 {
			verb = " runs here"
		}
		said = append([]string{strings.Join(app.Jobs, ", ") + verb}, said...)
	}
	if app.ToCheck > 0 {
		said = append(said, strconv.Itoa(app.ToCheck)+" to check")
	}
	return strings.Join(said, output.Dot)
}

var origins = map[envshow.Origin]string{
	envshow.OriginTemplate: "copied from its template",
	envshow.OriginMain:     "copied from main's",
	envshow.OriginParent:   "copied from the parent's",
	envshow.OriginFallback: "copied from main's: the parent has none",
	envshow.OriginOwn:      "the main checkout's own",
	envshow.OriginNone:     "not copied: its source has none",
}

func originWords(file envshow.File) string {
	said := origins[file.Origin]
	if file.Parent != "" {
		said = strings.Replace(said, "the parent", file.Parent, 1)
	}
	if file.Missing && file.Origin != envshow.OriginNone {
		said = "missing — `wtm env` rebuilds it, " + said
	}
	return said
}

var follows = map[envshow.Follows]string{
	envshow.FollowsPort:    "'s port",
	envshow.FollowsAddress: "'s address",
	envshow.FollowsValue:   "'s value",
	envshow.FollowsOwned:   "wtm's own",
	envshow.FollowsNothing: "follows nothing",
	envshow.FollowsCopied:  "copied as it is",
}

func followsWords(key envshow.Key) string {
	switch {
	case key.Follows == envshow.FollowsCopied && key.Status == env.KeyOrphan:
		return "in no source"
	case key.Follows == envshow.FollowsCopied && key.Here == nil:
		return "not here yet"
	}
	switch key.Follows {
	case envshow.FollowsPort, envshow.FollowsAddress, envshow.FollowsValue:
		return key.Job + follows[key.Follows]
	}
	return follows[key.Follows]
}

func value(v *string, hidden bool) string {
	switch {
	case v == nil:
		return none
	case hidden:
		return withheld
	}
	return cmp.Or(*v, `""`)
}

// hereValue is the key's value here, with what wtm writes when it is out of step.
func hereValue(key envshow.Key) string {
	here := value(key.Here, key.Withheld)
	if key.OutOfStep && key.Writes != nil {
		return here + " → " + *key.Writes
	}
	return here
}

func indented(rows [][]string) [][]string {
	for i := range rows {
		rows[i][0] = output.Indent + output.Indent + rows[i][0]
	}
	return rows
}
