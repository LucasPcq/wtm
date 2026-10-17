package output

import "strings"

var flags = map[string]string{
	"branches":      "<branches>",
	"from":          "--from",
	"env_from":      "--env-from",
	"isolation":     "--isolation",
	"fast_forward":  "--ff",
	"if_not_exists": "--if-not-exists",
	"remember":      "--remember",
}

var descriptions = map[string]string{
	"branches":      "The branches to create a worktree for; one that already exists is checked out as it is",
	"from":          "The branch a new branch starts from, or the parent recorded for wtm sync of one that already exists",
	"env_from":      "How the .env files of the new worktrees are provisioned: example (from its template), main (copied from the main checkout) or parent (from the source's worktree); defaults to config.toml's strategy",
	"isolation":     "How the new worktree stands against its source: isolated (its own ports, compose project and namespaces in shared services, in the .env and at run time) or verbatim (.env kept exactly as copied, run on its source's ports and data); defaults to run.toml's isolation, else isolated",
	"fast_forward":  "Fast-forward to origin first the branch each worktree starts from, when it is behind; one that has diverged or holds uncommitted changes starts as it is",
	"if_not_exists": "Succeed without a change when the worktree already exists (idempotent)",
}

func Describe(path string) string {
	return descriptions[path]
}

func Flag(path string) string {
	name, index, indexed := strings.Cut(path, "[")
	flag, known := flags[name]
	switch {
	case !known:
		return path
	case indexed:
		return flag + "[" + index
	}
	return flag
}
