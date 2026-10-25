package form

import (
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
)

// NewWorktree is what the words of the questions every command that creates a worktree asks say of it: where the command's new worktree takes its .env files from (EnvHelp), the strategy config.toml sets, the branches the fast-forward updates and whether several worktrees are created.
type NewWorktree struct {
	EnvHelp    string
	Configured project.EnvStrategy
	Behind     []string
	Many       bool
}

// Words are the words of the question of a new worktree at path, and report whether path is one of them.
func (n NewWorktree) Words(path string) (Words, bool) {
	switch path {
	case domain.PathEnvFrom:
		return Words{Flag: "--env-from", Label: "Env strategy", Help: n.EnvHelp, Options: map[string]OptionWords{
			string(project.RememberEnvConfig): {Label: "config default (" + string(n.Configured) + ")", Short: "config default"},
			string(project.EnvExample):        {Label: "example — from .env.example"},
			string(project.EnvMain):           {Label: "main — the main checkout's .env"},
			string(project.EnvParent):         {Label: "parent — the source worktree's .env"},
		}}, true
	case domain.PathIsolation:
		isolated := "isolated — its own ports and namespaces"
		if n.Many {
			isolated = "isolated — each its own ports and namespaces"
		}
		return Words{Flag: "--isolation", Label: "Isolation", Help: "How it stands against its source: isolated or verbatim; run.toml's isolation by default", Options: map[string]OptionWords{
			string(project.Isolated): {Label: isolated},
			string(project.Verbatim): {Label: "verbatim — the copied .env as is, one runs at a time"},
		}}, true
	case domain.PathFastForward:
		return Words{Flag: "--ff", Label: "Source update", Help: "Fast-forward the source branch to origin first when it is behind", Options: map[string]OptionWords{
			"true":  {Label: n.fastForwardLabel(), Short: "fast-forward to origin"},
			"false": {Label: "Keep it as-is", Short: "keep as-is"},
		}}, true
	}

	return Words{}, false
}

func (n NewWorktree) fastForwardLabel() string {
	if len(n.Behind) == 0 {
		return "Fast-forward to origin"
	}

	return "Fast-forward " + strings.Join(n.Behind, ", ") + " to origin"
}
