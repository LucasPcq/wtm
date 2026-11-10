package domain

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
)

// SpellHook is hook as a request's answer spells it: its command alone when it runs in the root and stops on a failure, else a JSON object (cmd, cwd, continue_on_error), which HookOf reads back.
func SpellHook(hook project.Hook) string {
	if hook.Cwd == "" && !hook.ContinueOnError {
		return hook.Cmd
	}

	var spelled bytes.Buffer
	encoder := json.NewEncoder(&spelled)
	encoder.SetEscapeHTML(false) // a command's && stays as typed
	_ = encoder.Encode(hook)
	return strings.TrimSuffix(spelled.String(), "\n")
}

// HookOf reads a hook as SpellHook spells it: a JSON object with a cmd, else the whole value as the command, so a shell group such as `{ make; }` stays a command.
func HookOf(value string) project.Hook {
	trimmed := strings.TrimSpace(value)
	if strings.HasPrefix(trimmed, "{") {
		decoder := json.NewDecoder(bytes.NewReader([]byte(trimmed)))
		decoder.DisallowUnknownFields()
		var hook project.Hook
		if err := decoder.Decode(&hook); err == nil && hook.Cmd != "" && !decoder.More() {
			return hook
		}
	}

	return project.Hook{Cmd: value}
}

// HooksOf reads each of values as HookOf does.
func HooksOf(values []string) []project.Hook {
	hooks := make([]project.Hook, 0, len(values))
	for _, value := range values {
		hooks = append(hooks, HookOf(value))
	}

	return hooks
}
