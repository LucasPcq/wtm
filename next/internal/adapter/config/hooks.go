package config

import (
	"fmt"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
)

// hooksFile is the [hooks] table of config.toml. Its entries stay untyped
// because a hook is written either as a string or as a {cmd, cwd} table.
type hooksFile struct {
	OnCreate []any `toml:"on_create"`
	OnClean  []any `toml:"on_clean"`
}

// hookTables are the keys the strict decoder leaves to decodeHooks, since the
// untyped entries always look undecoded to it.
var hookTables = []string{"hooks.on_create", "hooks.on_clean"}

// decode returns the on_create and on_clean hooks, or an error naming the list
// and the entry that is malformed.
func (h hooksFile) decode() ([]project.Hook, []project.Hook, error) {
	onCreate, err := decodeHooks(h.OnCreate)
	if err != nil {
		return nil, nil, fmt.Errorf("on_create: %w", err)
	}

	onClean, err := decodeHooks(h.OnClean)
	if err != nil {
		return nil, nil, fmt.Errorf("on_clean: %w", err)
	}

	return onCreate, onClean, nil
}

func decodeHooks(raw []any) ([]project.Hook, error) {
	hooks := make([]project.Hook, 0, len(raw))
	for i, entry := range raw {
		hook, err := decodeHook(entry)
		if err != nil {
			return nil, fmt.Errorf("hook[%d]: %w", i, err)
		}
		hooks = append(hooks, hook)
	}
	return hooks, nil
}

// decodeHook reads one hook written as a bare command string or as a table
// with a required cmd and an optional cwd and continue_on_error.
func decodeHook(raw any) (project.Hook, error) {
	if cmd, ok := raw.(string); ok {
		return project.Hook{Cmd: cmd}, nil
	}

	table, ok := raw.(map[string]any)
	if !ok {
		return project.Hook{}, fmt.Errorf("hook must be a string or {cmd, cwd}, got %T", raw)
	}
	cmd, ok := table["cmd"].(string)
	if !ok {
		return project.Hook{}, fmt.Errorf(`hook table requires a string "cmd" field`)
	}

	cwd, _ := table["cwd"].(string)
	continueOnError, _ := table["continue_on_error"].(bool)
	return project.Hook{Cmd: cmd, Cwd: cwd, ContinueOnError: continueOnError}, nil
}
