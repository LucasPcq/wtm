package rules

import (
	"path"
	"slices"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// IsOwnedEnvKey says whether wtm derives this key from the worktree itself,
// which is what takes it out of the reconciliation's verdict: a value that
// differs per worktree by construction is neither drift nor a conflict.
func IsOwnedEnvKey(key string) bool {
	return slices.Contains(domain.WtmOwnedEnvKeys, key)
}

type OwnedEnvWritesParams struct {
	Config   domain.RunConfig
	EnvFiles []domain.EnvFile
	Values   map[string]string
}

// OwnedEnvWrites places each owned key in the .env of the directory the command
// reading it runs from. Compose interpolates the .env of its own project
// directory, so a name written anywhere else would never be read.
func OwnedEnvWrites(params OwnedEnvWritesParams) []domain.EnvOwnedEntry {
	var writes []domain.EnvOwnedEntry
	for _, target := range OwnedEnvTargets(OwnedEnvTargetsParams{Config: params.Config, EnvFiles: params.EnvFiles}) {
		for _, key := range domain.WtmOwnedEnvKeys {
			value := params.Values[key]
			if value == "" {
				continue
			}
			writes = append(writes, domain.EnvOwnedEntry{File: target, Key: key, Value: value})
		}
	}
	return writes
}

type OwnedEnvTargetsParams struct {
	Config   domain.RunConfig
	EnvFiles []domain.EnvFile
}

// OwnedEnvTargets are the provisioned .env files a compose stack reads.
func OwnedEnvTargets(params OwnedEnvTargetsParams) []string {
	byDir := make(map[string]string, len(params.EnvFiles))
	for _, file := range params.EnvFiles {
		byDir[ScriptJobCwd(path.Dir(file.Target))] = file.Target
	}

	var targets []string
	for _, dir := range ComposeProjectDirs(params.Config) {
		target, declared := byDir[dir]
		if !declared {
			continue
		}
		targets = append(targets, target)
	}
	return targets
}

// ComposeProjectDirs are the directories a compose stack is started from, which
// is where the .env it interpolates lives.
func ComposeProjectDirs(cfg domain.RunConfig) []string {
	var dirs []string
	seen := map[string]bool{}
	for _, job := range cfg.Jobs {
		if !runsCompose(job) {
			continue
		}
		dir := ScriptJobCwd(job.Cwd)
		if seen[dir] {
			continue
		}
		seen[dir] = true
		dirs = append(dirs, dir)
	}
	return dirs
}

func runsCompose(job domain.JobConfig) bool {
	for _, cmd := range []string{job.Cmd, job.Stop} {
		if strings.Contains(cmd, domain.ComposeCmdSpaced) || strings.Contains(cmd, domain.ComposeCmdHyphened) {
			return true
		}
	}
	return false
}

type UpsertEnvPairParams struct {
	Lines []domain.EnvLine
	Key   string
	Value string
}

// UpsertEnvPair sets Key to Value, in place when the document already holds it
// and inserted otherwise. Raw is cleared on a mutated line so RenderEnv re-emits
// it from Key and Value.
func UpsertEnvPair(params UpsertEnvPairParams) (lines []domain.EnvLine, changed bool) {
	lines = params.Lines
	for i, line := range lines {
		if line.Kind != domain.EnvLinePair || line.Key != params.Key {
			continue
		}
		if line.Value == params.Value {
			return lines, false
		}
		out := slices.Clone(lines)
		out[i].Value = params.Value
		out[i].Raw = ""
		return out, true
	}
	// Inserted after the last line that holds something, not at the very end: a
	// parsed document keeps its trailing newline as a final blank line, and
	// appending past it drops the newline and inserts a blank line instead.
	pair := domain.EnvLine{Kind: domain.EnvLinePair, Key: params.Key, Value: params.Value}
	at := len(lines)
	for at > 0 && lines[at-1].Kind == domain.EnvLineBlank {
		at--
	}
	out := make([]domain.EnvLine, 0, len(lines)+1)
	out = append(out, lines[:at]...)
	out = append(out, pair)
	return append(out, lines[at:]...), true
}
