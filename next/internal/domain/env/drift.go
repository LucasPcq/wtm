package env

import (
	"slices"

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

// KeyDrift counts the drift of params.Child (Keys): a key its source gives a value it lacks is no drift, since reconciling adds it.
func KeyDrift(params DriftParams) Drift {
	var drift Drift
	for _, key := range Keys(params) {
		switch key.Status {
		case KeyMissing:
			drift.Missing++
		case KeyConflict:
			drift.Conflicting++
		case KeyOrphan:
			drift.Orphan++
		}
	}

	return drift
}

// KeyStatus is how one key of a worktree's .env file stands against its sources.
type KeyStatus string

// The statuses of a key: in step with its source; absent and given a value by its source (Add); expected with no value anywhere (Missing); holding another value than its source's (Conflict); in no source (Orphan).
const (
	KeyInStep   KeyStatus = "in_step"
	KeyAdd      KeyStatus = "add"
	KeyMissing  KeyStatus = "missing"
	KeyConflict KeyStatus = "conflict"
	KeyOrphan   KeyStatus = "orphan"
)

// KeyStatuses returns every KeyStatus.
func KeyStatuses() []KeyStatus {
	return []KeyStatus{KeyInStep, KeyAdd, KeyMissing, KeyConflict, KeyOrphan}
}

// A Key is one key of a worktree's .env file against its sources: its status, its value in the file (none when absent) and its source's (none when no source gives one).
type Key struct {
	Name   string
	Status KeyStatus
	Value  string
	Source string
}

// Keys returns every key of params.Child and of its sources but those wtm owns, the child's in file order, then the others in the template's, the parent's, then the main's order. An expected key is one of the template, the parent's or the main's; its value is the first non-empty one of the parent's, then the main's.
func Keys(params DriftParams) []Key {
	child, template, parent, main := Values(params.Child), Values(params.Template), Values(params.Parent), Values(params.Main)

	var keys []Key
	seen := map[string]bool{}
	for _, lines := range [][]dotenv.Line{params.Child, params.Template, params.Parent, params.Main} {
		for _, line := range lines {
			name := line.Key
			if line.Kind != dotenv.Pair || seen[name] || params.Owned[name] {
				continue
			}
			seen[name] = true

			value, inChild := child[name]
			_, inTemplate := template[name]
			_, inParent := parent[name]
			_, inMain := main[name]
			key := Key{Name: name, Status: KeyInStep, Value: value, Source: cascade(name, parent, main)}
			switch {
			case inChild && !inTemplate && !inParent && !inMain:
				key.Status = KeyOrphan
			case inChild && key.Source != "" && key.Source != value:
				key.Status = KeyConflict
			case !inChild && key.Source == "":
				key.Status = KeyMissing
			case !inChild:
				key.Status = KeyAdd
			}
			keys = append(keys, key)
		}
	}

	return keys
}

// ApplyKeys returns lines with each of keys settled on its source: an addition added after the last line holding something, a conflict given its source's value, an orphan dropped; a key in step or missing changes nothing.
func ApplyKeys(lines []dotenv.Line, keys []Key) []dotenv.Line {
	for _, key := range keys {
		switch key.Status {
		case KeyAdd, KeyConflict:
			lines, _ = dotenv.Set(dotenv.SetParams{Lines: lines, Key: key.Name, Value: key.Source})
		case KeyOrphan:
			lines = slices.DeleteFunc(slices.Clone(lines), func(line dotenv.Line) bool { return line.Kind == dotenv.Pair && line.Key == key.Name })
		}
	}

	return lines
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

// Values are the values of lines by key, the last of a key winning as a .env file reads.
func Values(lines []dotenv.Line) map[string]string {
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
