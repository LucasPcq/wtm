package domain

import (
	"bytes"
	"encoding/json"
	"strconv"
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

// CodeNotAHook is the field-error code of an entry written as a JSON object that is no hook: a key it does not know, or no cmd.
const CodeNotAHook = "not_a_hook"

// HooksOf reads each of values, the entries of the field at path, as HookOf does, refusing an entry written as a JSON object that is no hook, so a mistyped key is not run as a command.
func HooksOf(path string, values []string) ([]project.Hook, error) {
	hooks := make([]project.Hook, 0, len(values))
	var problems []FieldError
	for index, value := range values {
		hook := HookOf(value)
		trimmed := strings.TrimSpace(value)
		if hook.Cmd == value && strings.HasPrefix(trimmed, "{") && json.Valid([]byte(trimmed)) {
			problems = append(problems, FieldError{Path: path + "[" + strconv.Itoa(index) + "]", Code: CodeNotAHook, Template: "{path} is JSON but no hook: it takes cmd, cwd and continue_on_error"})
		}
		hooks = append(hooks, hook)
	}
	if len(problems) > 0 {
		return nil, Invalid(problems)
	}

	return hooks, nil
}
