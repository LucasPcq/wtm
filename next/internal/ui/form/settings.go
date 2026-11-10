package form

import (
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
)

// Settings is what the words of config.toml's settings say of them, which init asks and a change to the settings writes: the .env files known, each with its template, and the hooks the repository suggests for each list.
type Settings struct {
	EnvFiles []project.EnvFile
	OnCreate []project.FoundHook
	OnClean  []project.FoundHook
}

// Words are the words of the setting at path, and report whether path is one of them.
func (s Settings) Words(path string) (Words, bool) {
	switch path {
	case domain.PathBasePath:
		return Words{Flag: "--base-path", Label: "Worktrees folder", Help: "Where new worktrees go, relative to the repository; outside it keeps the main checkout clean"}, true
	case domain.PathBaseBranch:
		return Words{Flag: "--base-branch", Label: "Base branch", Help: "The branch new worktrees start from by default"}, true
	case domain.PathEnvStrategy:
		return Words{Flag: "--env-strategy", Label: ".env from", Help: "Where a new worktree's .env files come from: a fresh checkout has none of your local ones", Options: map[string]OptionWords{
			string(project.EnvExample): {Label: "Copy the template (.env.example → .env)", Short: "the template"},
			string(project.EnvMain):    {Label: "Copy main's .env", Short: "main's"},
			string(project.EnvParent):  {Label: "Copy the .env of the worktree it starts from", Short: "the parent's"},
			string(project.EnvNone):    {Label: "Don't copy them", Short: "none"},
		}}, true
	case domain.PathEnvFiles:
		return Words{Flag: "--env-file", Label: ".env files", Help: "The .env files every new worktree gets", Empty: "Pick at least one .env file, or none for where they come from", Options: fileWords(s.EnvFiles)}, true
	case domain.PathOnCreate:
		return Words{Flag: "--on-create", Label: "On create", Help: hooksHelp("Commands run in a worktree once it is created, such as installing its dependencies", s.OnCreate)}, true
	case domain.PathOnClean:
		return Words{Flag: "--on-clean", Label: "On clean", Help: hooksHelp("Commands run in a worktree before it is removed, such as stopping what it started", s.OnClean)}, true
	}

	return Words{}, false
}

func fileWords(files []project.EnvFile) map[string]OptionWords {
	said := make(map[string]OptionWords, len(files))
	for _, file := range files {
		label := file.Target
		if file.Template != "" {
			label += "  ← " + file.Template
		}
		said[file.Target] = OptionWords{Label: label}
	}

	return said
}

// hooksHelp is a list's help, followed by the hooks the repository suggests and where each was found.
func hooksHelp(help string, found []project.FoundHook) string {
	if len(found) == 0 {
		return help
	}
	said := make([]string, 0, len(found))
	for _, hook := range found {
		said = append(said, hook.Cmd+" (from "+hook.File+")")
	}

	return help + "\nFound: " + strings.Join(said, ", ")
}
