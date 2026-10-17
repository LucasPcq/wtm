package config

import (
	"fmt"

	"github.com/LucasPcq/wtm/next/internal/wtm/project"
)

type hooksFile struct {
	OnCreate []any `toml:"on_create"`
	OnClean  []any `toml:"on_clean"`
}

var hookTables = []string{"hooks.on_create", "hooks.on_clean"}

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
