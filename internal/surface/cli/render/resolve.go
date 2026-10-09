package output

import "io"

// ResolveJSON is the payload emitted by `resolve --output json`: the resolved
// worktree path and the branch backing it.
type ResolveJSON struct {
	Path   string `json:"path"`
	Branch string `json:"branch"`
}

// WriteResolveJSON writes the resolve payload as JSON.
func WriteResolveJSON(w io.Writer, payload ResolveJSON) error {
	return encodeJSON(w, payload)
}
