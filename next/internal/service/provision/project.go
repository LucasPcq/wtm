package provision

import "github.com/LucasPcq/wtm/next/internal/domain/worktree"

// ProjectOf is what Run needs of the project, as repo holds it, with the path of wtm's state.
func ProjectOf(repo worktree.Repository, stateDir string) Project {
	return Project{
		MainPath: repo.MainPath,
		StateDir: stateDir,
		EnvFiles: repo.Config.Env.Files,
		OnCreate: repo.Config.OnCreate,
		Run:      repo.Run,
		RunErr:   repo.RunErr,
		Global:   repo.Global,
	}
}
