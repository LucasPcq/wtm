package settle

import "github.com/LucasPcq/wtm/next/internal/wtm/run"

type worktree struct {
	Addressing run.Addressing
	Jobs       map[string]run.Job
	Slug       string
	Project    string
	PublicPort int
	Ordinal    int
	Offset     int
	Shared     map[string]bool
}

func (w worktree) names() bool {
	return w.Addressing == run.AddressNames && w.PublicPort > 0
}

func (w worktree) jobLabel(job string) string {
	return jobHostLabel(w.Jobs[job])
}
