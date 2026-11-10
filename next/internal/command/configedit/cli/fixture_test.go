package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/config"
	"github.com/LucasPcq/wtm/next/internal/command/configedit"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/settings"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"

	"github.com/stretchr/testify/require"
)

// acme is config.toml as init writes it for acme.
const acme = `schema = 1

[worktrees]
base_path = "../.trees"
base_branch = "main"

[env]
strategy = "example"
`

// A world is a repository's config.toml before the editor opens it and as the editor leaves it.
type world struct {
	Config string
	Edited string
}

func (w world) env() uicli.Env {
	return uicli.Env{Cwd: "/acme", ProjectDir: "/acme"}
}

// opener is config edit over the state directory dir, holding w's config.toml, its editor writing w.Edited.
func (w world) opener(t *testing.T, dir string) func(context.Context, string) (configedit.UseCase, error) {
	path := filepath.Join(dir, "config.toml")
	require.NoError(t, os.WriteFile(path, []byte(w.Config), 0o644))
	editor := func(_ context.Context, at string) error { return os.WriteFile(at, []byte(w.Edited), 0o644) }
	return func(context.Context, string) (configedit.UseCase, error) {
		return configedit.UseCase{Path: path, Editor: editor, Settings: settings.Service{Store: config.Project{StateDir: dir}}, Git: branches{}, Init: &domain.FollowUp{Command: domain.CommandInit}}, nil
	}
}

// branches are acme's: main.
type branches struct{}

func (branches) Branches(context.Context) (worktree.Branches, error) {
	return worktree.Branches{Local: []worktree.Branch{{Name: "main"}}}, nil
}
