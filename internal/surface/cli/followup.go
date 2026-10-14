package cli

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/surface/cli/render"
)

const (
	wordSeparator = " "
	pairFmt       = "%s=%s"
	falseFmt      = "%s=false"
	shellSpecial  = " \t\n'\"$`\\|&;<>()*?[]{}~#!"
	quoteFmt      = "'%s'"
	quoteEscape   = `'\''`
	unspelledFmt  = "the request for %s is no object: %w"
)

// followUps spells each follow-up as the command line that runs it.
func (b *binding) followUps(followUps []kernel.FollowUp) ([]render.NextStepParams, error) {
	steps := make([]render.NextStepParams, 0, len(followUps))
	for _, followUp := range followUps {
		line, err := b.engine.commandLine(lineParams{Target: followUp.Command, Request: followUp.Request})
		if err != nil {
			return nil, err
		}
		steps = append(steps, render.NextStepParams{Command: line, Note: b.engine.catalog.Message(followUp.Code, followUp.Params)})
	}
	return steps, nil
}

type rerunParams struct {
	Request json.RawMessage
	Flag    string
	Value   string
}

// rerun is this command's line for the same request, plus one engine flag.
func (b *binding) rerun(params rerunParams) (string, error) {
	line, err := b.engine.commandLine(lineParams{Target: b.name, Request: params.Request})
	if err != nil {
		return "", err
	}
	words := []string{line, flagPrefix + params.Flag}
	if params.Value != "" {
		words = append(words, params.Value)
	}
	return strings.Join(words, wordSeparator), nil
}

type lineParams struct {
	Target  string
	Request json.RawMessage
}

// commandLine spells a request for the command target: through its binding
// when it is served here, as kebab-case flags otherwise. A request that is not
// an object is an error of the command that proposed it.
func (e *Engine) commandLine(params lineParams) (string, error) {
	var request map[string]any
	if len(params.Request) > 0 {
		if err := json.Unmarshal(params.Request, &request); err != nil {
			return "", kernel.Internal(kernel.Problem{Code: kernel.CodeInternal, Cause: fmt.Errorf(unspelledFmt, params.Target, err)})
		}
	}
	target := e.bindings[params.Target]
	var args, flags []string
	for _, leaf := range leaves(leavesParams{Target: target, Object: request}) {
		if target != nil && leaf.Path == target.positional {
			args = append(args, quoteAll(valueWords(leaf.Value))...)
			continue
		}
		flags = append(flags, flagWords(flagWordsParams{Flag: flagSpelling(target, leaf.Path), Value: leaf.Value})...)
	}
	words := append([]string{commandPath(target, params.Target)}, args...)
	return strings.Join(append(words, flags...), wordSeparator), nil
}

func commandPath(target *binding, name string) string {
	if target == nil || target.cmd == nil {
		return domain.AppName + wordSeparator + name
	}
	return target.cmd.CommandPath()
}

func flagSpelling(target *binding, path string) string {
	if target == nil {
		return flagPrefix + kebab(path)
	}
	return flagPrefix + target.flagOf(path).Name
}

type leavesParams struct {
	Target *binding
	Prefix string
	Object map[string]any
}

// leaves walks the request's objects in key order down to the values a flag
// takes; a decisions field is a value, not an object to walk.
func leaves(params leavesParams) []entry {
	keys := make([]string, 0, len(params.Object))
	for key := range params.Object {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	var found []entry
	for _, key := range keys {
		path := params.Prefix + key
		object, nested := params.Object[key].(map[string]any)
		if nested && !isDecisions(params.Target, path) {
			found = append(found, leaves(leavesParams{Target: params.Target, Prefix: path + pathSeparator, Object: object})...)
			continue
		}
		found = append(found, entry{Path: path, Value: params.Object[key]})
	}
	return found
}

func isDecisions(target *binding, path string) bool {
	if target == nil {
		return false
	}
	spec, known := target.spec(path)
	return known && spec.Type == kernel.FieldDecisions
}

type flagWordsParams struct {
	Flag  string
	Value any
}

func flagWords(params flagWordsParams) []string {
	switch value := params.Value.(type) {
	case bool:
		if value {
			return []string{params.Flag}
		}
		return []string{fmt.Sprintf(falseFmt, params.Flag)}
	case map[string]any:
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		var out []string
		for _, key := range keys {
			out = append(out, params.Flag, quote(fmt.Sprintf(pairFmt, key, fmt.Sprint(value[key]))))
		}
		return out
	}
	var out []string
	for _, word := range valueWords(params.Value) {
		out = append(out, params.Flag, quote(word))
	}
	return out
}

func valueWords(value any) []string {
	list, ok := value.([]any)
	if !ok {
		return []string{fmt.Sprint(value)}
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		out = append(out, fmt.Sprint(item))
	}
	return out
}

func quoteAll(words []string) []string {
	out := make([]string, 0, len(words))
	for _, word := range words {
		out = append(out, quote(word))
	}
	return out
}

// quote leaves a plain word bare and single-quotes anything a shell would read.
func quote(word string) string {
	if word != "" && !strings.ContainsAny(word, shellSpecial) {
		return word
	}
	return fmt.Sprintf(quoteFmt, strings.ReplaceAll(word, "'", quoteEscape))
}
