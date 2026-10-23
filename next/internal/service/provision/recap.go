package provision

import (
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
)

// The notes of the values RecapRows shows.
const (
	NoteConfigDefault = "config_default" // the env strategy follows config.toml's
	NoteNone          = "none"           // no source is fast-forwarded
	NoteRunUnreadable = "run_unreadable" // the isolation is settled without run.toml, which could not be read
)

// RecapParams are the answers of a new worktree as a plan shows them: the env strategy and whether it follows config.toml's, the isolation planned and whether run.toml could not be read, the sources to fast-forward, and the marks of their answers.
type RecapParams struct {
	EnvFrom       project.EnvStrategy
	FollowsConfig bool
	Isolation     project.Isolation
	RunUnreadable bool
	FastForward   []string
	Marks         domain.Marks
}

// RecapRows are the rows params shows: Env when a strategy is planned, Isolation when one is (marked when run.toml could not be read), Fast-forward when a source is or its answer is marked. A marked env strategy that follows config.toml's reads config default, since that, not the strategy, is what is remembered.
func RecapRows(params RecapParams) []domain.RecapRow {
	var rows []domain.RecapRow
	if params.EnvFrom != "" {
		rows = append(rows, envRow(params))
	}
	if params.Isolation != "" {
		rows = append(rows, isolationRow(params))
	}

	ff := domain.RecapRow{Key: domain.PathFastForward, Label: "Fast-forward", Path: domain.PathFastForward, Mark: params.Marks.Of(domain.PathFastForward), Values: domain.RecapValues(params.FastForward)}
	switch {
	case len(ff.Values) > 0:
		rows = append(rows, ff)
	case ff.Mark != "":
		ff.Values = []domain.RecapValue{{Note: NoteNone, Text: "keep as-is"}}
		rows = append(rows, ff)
	}

	return rows
}

func isolationRow(params RecapParams) domain.RecapRow {
	isolation := string(params.Isolation)
	row := domain.RecapRow{Key: domain.PathIsolation, Label: "Isolation", Path: domain.PathIsolation, Mark: params.Marks.Of(domain.PathIsolation), Values: domain.RecapValues([]string{isolation})}
	if params.RunUnreadable {
		row.Values[0].Note, row.Values[0].Text = NoteRunUnreadable, isolation+" (run.toml unreadable)"
	}

	return row
}

func envRow(params RecapParams) domain.RecapRow {
	strategy := string(params.EnvFrom)
	row := domain.RecapRow{Key: domain.PathEnvFrom, Label: "Env", Path: domain.PathEnvFrom, Mark: params.Marks.Of(domain.PathEnvFrom), Values: domain.RecapValues([]string{strategy})}
	if params.FollowsConfig && row.Mark != "" {
		row.Values[0].Note, row.Values[0].Text = NoteConfigDefault, "config default ("+strategy+")"
	}

	return row
}
