package worktree

import "time"

// A Commit is one commit of a worktree's history: its short sha, its subject, its author and when it was committed.
type Commit struct {
	SHA     string    `json:"sha"`
	Subject string    `json:"subject"`
	Author  string    `json:"author"`
	At      time.Time `json:"at"`
}

// A Diffstat measures a worktree's uncommitted work against its last commit: the tracked files changed in its index (Staged) and in its working tree but not staged (Unstaged), a file in both counted in both, and the lines inserted and deleted, index included.
type Diffstat struct {
	Staged     int
	Unstaged   int
	Insertions int
	Deletions  int
}
