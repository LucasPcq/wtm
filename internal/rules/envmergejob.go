package rules

import (
	"strings"
)

type MergeEnvParams struct {
	Env []string
	// Clear names variables dropped before Overrides is applied, for a caller
	// whose own environment cannot be trusted to describe the run.
	Clear     []string
	Overrides map[string]string
}

// MergeEnv layers overrides onto an environment, replacing rather than
// appending so a key appears once whatever the caller inherited.
func MergeEnv(params MergeEnvParams) []string {
	merged := params.Env
	for _, key := range params.Clear {
		merged = withoutEnv(merged, key)
	}
	for _, key := range sortedKeys(params.Overrides) {
		merged = append(withoutEnv(merged, key), key+"="+params.Overrides[key])
	}
	return merged
}

// LookupEnv reads a key out of an environment slice.
func LookupEnv(env []string, key string) (string, bool) {
	prefix := key + "="
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			return strings.TrimPrefix(entry, prefix), true
		}
	}
	return "", false
}

// SetEnv replaces any existing occurrence of key, so the value wins over what
// the caller inherited.
func SetEnv(env []string, key, value string) []string {
	return append(withoutEnv(env, key), key+"="+value)
}

func withoutEnv(env []string, key string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	for _, entry := range env {
		if !strings.HasPrefix(entry, prefix) {
			out = append(out, entry)
		}
	}
	return out
}
