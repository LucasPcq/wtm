package env

import (
	"github.com/LucasPcq/wtm/next/internal/domain/dotenv"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
)

// Drift counts how the keys of a worktree's .env file stand against its sources: Missing are expected keys it lacks with no value to take from any source, Conflicting keys whose value differs from the source's, Orphan keys no source has.
type Drift struct {
	Missing     int `json:"missing"`
	Conflicting int `json:"conflicting"`
	Orphan      int `json:"orphan"`
}

// DriftParams are a worktree's .env file (Child) and what it is compared with: its template, which names keys but gives no value, then the parent worktree's copy and the main worktree's, the values in that order; Owned are the keys wtm writes itself, which differ from every source by construction.
type DriftParams struct {
	Child    []dotenv.Line
	Template []dotenv.Line
	Parent   []dotenv.Line
	Main     []dotenv.Line
	Owned    map[string]bool
}

// KeyDrift counts the drift of params.Child. An expected key is one of the template, the parent's or the main's; its value is the first non-empty one of the parent's, then the main's.
func KeyDrift(params DriftParams) Drift {
	child, template, parent, main := pairs(params.Child), pairs(params.Template), pairs(params.Parent), pairs(params.Main)

	var drift Drift
	seen := map[string]bool{}
	for _, lines := range [][]dotenv.Line{params.Child, params.Template, params.Parent, params.Main} {
		for _, line := range lines {
			key := line.Key
			if line.Kind != dotenv.Pair || seen[key] || params.Owned[key] {
				continue
			}
			seen[key] = true

			value, inChild := child[key]
			_, inTemplate := template[key]
			_, inParent := parent[key]
			_, inMain := main[key]
			source := cascade(key, parent, main)
			switch {
			case inChild && !inTemplate && !inParent && !inMain:
				drift.Orphan++
			case inChild && source != "" && source != value:
				drift.Conflicting++
			case !inChild && source == "":
				drift.Missing++
			}
		}
	}

	return drift
}

// Owned are the keys wtm writes in the .env file target: the variables it sets for every job and the keys run.toml links into that file.
func Owned(run project.Run, target string) map[string]bool {
	owned := map[string]bool{}
	for _, key := range JobKeys() {
		owned[key] = true
	}
	for _, link := range run.EnvPorts {
		if link.File == target {
			owned[link.Key] = true
		}
	}
	for _, link := range run.EnvValues {
		if link.File == target {
			owned[link.Key] = true
		}
	}

	return owned
}

// pairs are the values of lines by key, the last of a key winning as a .env file reads.
func pairs(lines []dotenv.Line) map[string]string {
	values := map[string]string{}
	for _, line := range lines {
		if line.Kind == dotenv.Pair {
			values[line.Key] = line.Value
		}
	}

	return values
}

// cascade is the value of key in the first source that gives it one, the parent's then the main's; empty when neither does.
func cascade(key string, sources ...map[string]string) string {
	for _, source := range sources {
		if value := source[key]; value != "" {
			return value
		}
	}

	return ""
}
