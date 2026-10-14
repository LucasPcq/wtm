package runenv

import (
	"context"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/store"
)

type ApplyParams struct {
	Files  store.EnvFiles
	Writes EnvWrites
}

type Applied struct {
	Plan     domain.EnvPortPlan
	Files    []string
	Unlocked bool
}

func Apply(ctx context.Context, params ApplyParams) (Applied, error) {
	writes := params.Writes
	if writes.Empty() {
		return Applied{}, nil
	}
	read := map[string][]domain.EnvLine{}
	var applied Applied
	for _, file := range targetsOf(writes) {
		written, err := params.Files.Edit(ctx, store.EnvEdit{
			Path:   filepath.Join(writes.WorktreePath, file),
			Create: ownsKeyIn(ownsKeyInParams{Owned: writes.Owned, File: file}),
			Edit: func(lines []domain.EnvLine) []domain.EnvLine {
				read[file] = lines
				return edited(editedParams{Writes: writes, File: file, Lines: lines})
			},
		})
		if err != nil {
			return Applied{}, err
		}
		if written.Changed {
			applied.Files = append(applied.Files, file)
		}
		applied.Unlocked = applied.Unlocked || written.Unlocked
	}
	applied.Plan = planOf(planOfParams{Writes: writes, Links: writes.Links, Lines: read})
	applied.Plan.Owned = ownedOf(ownedOfParams{Owned: writes.Owned, Lines: read})
	applied.Plan.Applied = true
	return applied, nil
}

type editedParams struct {
	Writes EnvWrites
	File   string
	Lines  []domain.EnvLine
}

func edited(params editedParams) []domain.EnvLine {
	plan := planOf(planOfParams{
		Writes: params.Writes,
		Links:  linksIn(linksInParams{Links: params.Writes.Links, File: params.File}),
		Lines:  map[string][]domain.EnvLine{params.File: params.Lines},
	})
	// The ports before the owned keys, as wtm env does: both act on the same document.
	lines := rules.ApplyEnvPorts(params.Lines, plan.Entries)
	for _, entry := range params.Writes.Owned {
		if entry.File == params.File {
			lines, _ = rules.UpsertEnvPair(rules.UpsertEnvPairParams{Lines: lines, Key: entry.Key, Value: entry.Value})
		}
	}
	return lines
}

type planOfParams struct {
	Writes EnvWrites
	Links  []domain.EnvPortLink
	Lines  map[string][]domain.EnvLine
}

func planOf(params planOfParams) domain.EnvPortPlan {
	writes := params.Writes
	return rules.PlanEnvPorts(rules.PlanEnvPortsParams{
		Links:   params.Links,
		Bases:   writes.Bases,
		Origins: writes.Origins,
		Block:   writes.Block,
		Offset:  writes.Offset,
		Lines:   params.Lines,
		Shared:  writes.Shared,
	})
}

type ownedOfParams struct {
	Owned []domain.EnvOwnedEntry
	Lines map[string][]domain.EnvLine
}

func ownedOf(params ownedOfParams) []domain.EnvOwnedEntry {
	if len(params.Owned) == 0 {
		return nil
	}
	lines := map[string][]domain.EnvLine{}
	for file, read := range params.Lines {
		lines[file] = read
	}
	owned := make([]domain.EnvOwnedEntry, 0, len(params.Owned))
	for _, entry := range params.Owned {
		updated, changed := rules.UpsertEnvPair(rules.UpsertEnvPairParams{Lines: lines[entry.File], Key: entry.Key, Value: entry.Value})
		lines[entry.File] = updated
		entry.Changed = changed
		owned = append(owned, entry)
	}
	return owned
}

func targetsOf(writes EnvWrites) []string {
	var files []string
	seen := map[string]bool{}
	add := func(file string) {
		if !seen[file] {
			seen[file] = true
			files = append(files, file)
		}
	}
	for _, link := range writes.Links {
		add(link.File)
	}
	for _, entry := range writes.Owned {
		add(entry.File)
	}
	return files
}

type linksInParams struct {
	Links []domain.EnvPortLink
	File  string
}

func linksIn(params linksInParams) []domain.EnvPortLink {
	var links []domain.EnvPortLink
	for _, link := range params.Links {
		if link.File == params.File {
			links = append(links, link)
		}
	}
	return links
}

type ownsKeyInParams struct {
	Owned []domain.EnvOwnedEntry
	File  string
}

func ownsKeyIn(params ownsKeyInParams) bool {
	for _, entry := range params.Owned {
		if entry.File == params.File {
			return true
		}
	}
	return false
}
