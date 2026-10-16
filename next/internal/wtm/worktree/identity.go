package worktree

import "github.com/LucasPcq/wtm/next/internal/wtm/run"

type Identity struct {
	Branch    string        `json:"branch"`
	Path      string        `json:"path"`
	Parent    string        `json:"parent"`
	Ordinal   *int          `json:"ordinal"`
	Isolation run.Isolation `json:"isolation"`
	Main      bool          `json:"is_main"`
	CreatedAt string        `json:"created_at"`
}
