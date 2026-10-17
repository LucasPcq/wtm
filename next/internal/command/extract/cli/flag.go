package cli

import "github.com/LucasPcq/wtm/next/internal/ui/output"

var extractFlags = output.Flags{
	"source":       "<source>",
	"files":        "--files",
	"to":           "--to",
	"from":         "--from",
	"isolation":    "--isolation",
	"fast_forward": "--ff",
	"keep":         "--keep",
	"on_conflict":  "--on-conflict",
	"remember":     "--remember",
}

var descriptions = map[string]string{
	"files":        "Files to extract, or a directory to take everything below it (skips interactive selection)",
	"to":           "Target worktree branch; created if it does not exist",
	"isolation":    "How the new worktree stands against its source: isolated (its own ports, compose project and namespaces in shared services, in the .env and at run time) or verbatim (.env kept exactly as copied, run on its source's ports and data); defaults to run.toml's isolation, else isolated",
	"from":         "Parent branch when creating the target worktree",
	"fast_forward": "Fast-forward the parent branch to origin before creating the target (answers the wizard's question; skipped when it has diverged)",
	"keep":         "Copy instead of move (keep the changes in the source)",
	"on_conflict":  "On conflict: abort (default) or resolve (write conflict markers in the target)",
}
