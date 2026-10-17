package worktree

func live(branches ...string) []Worktree {
	worktrees := []Worktree{{Path: "/repo", Main: true, State: OnBranch{Branch: "main"}}}
	for _, branch := range branches {
		worktrees = append(worktrees, Worktree{Path: "/trees/" + branch, State: OnBranch{Branch: branch}})
	}

	return worktrees
}

func from(source string) Meta {
	return Meta{SourceBranch: source}
}

func at(source, createdAt string) Meta {
	return Meta{SourceBranch: source, CreatedAt: createdAt}
}
