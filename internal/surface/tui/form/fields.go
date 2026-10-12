package form

import (
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/surface/tui/components"
)

type fieldParams struct {
	Index   int
	Request json.RawMessage
	Form    kernel.Form[json.RawMessage]
}

// model draws a field by the shape of its value, offering the value State
// gives it. A field whose choices are still loading shows none yet.
func (s *session) model(params fieldParams) any {
	spec := s.fields[params.Index]
	value := stateAt(params.Form, spec.Path).Value
	choices := s.choicesFor(params)
	validate := func(candidate kernel.Value) error {
		return s.check(checkParams{Index: params.Index, Request: params.Request, Value: candidate})
	}
	switch spec.Type {
	case kernel.FieldText:
		return components.NewTextInput(components.NewTextInputParams{
			Title: titleOf(spec), Description: spec.Description, Default: textOf(value),
			Validate: func(typed string) error { return validate(kernel.Text(typed)) },
		})
	case kernel.FieldBool:
		yes, _ := value.(kernel.Bool)
		return components.NewConfirm(components.NewConfirmParams{Title: titleOf(spec), Description: spec.Description, DefaultYes: bool(yes)})
	case kernel.FieldMultiSelect:
		return components.NewMultiSelect(components.NewMultiSelectParams{
			Title: titleOf(spec), Description: spec.Description, Items: s.multiItems(choices, list(value)),
			Validate: func(values []string) error { return validate(kernel.List(values)) },
		})
	case kernel.FieldReorder:
		return components.NewReorderList(components.NewReorderListParams{Title: titleOf(spec), Description: spec.Description, Items: reorderItems(choices, list(value))})
	case kernel.FieldTextList:
		return s.textList(textListParams{fieldParams: params, Choices: choices, Entries: list(value)})
	case kernel.FieldDecisions:
		return components.NewDecisionList(components.NewDecisionListParams{
			Title: titleOf(spec), Description: spec.Description, Rows: decisionRows(choices, value),
			Validate: func(values map[string]string) error { return validate(kernel.Decisions(values)) },
		})
	}
	return s.selectList(selectParams{fieldParams: params, Choices: choices, Value: value})
}

type selectParams struct {
	fieldParams
	Choices kernel.Choices
	Value   kernel.Value
}

// selectList offers "always use this answer" on a rememberable field of a
// command that remembers; asked again on --ask, it opens ticked on the answer
// the repository remembers.
func (s *session) selectList(params selectParams) components.SelectListModel {
	spec := s.fields[params.Index]
	start := textOf(params.Value)
	if start == "" {
		start = params.Choices.Pinned
	}
	list := components.NewSelectListParams{Title: titleOf(spec), Description: spec.Description, Items: s.selectItems(params.Choices), Start: start}
	if !s.remember || !spec.Rememberable {
		return components.NewSelectList(list)
	}
	list.Toggle = domain.RememberToggleLabel
	list.ToggleOn = s.ticked(spec.Path)
	if recalled, ok := s.recalled(params.Request, spec.Path); ok {
		list.ToggleOn, list.Start = true, textOf(recalled)
	}
	return components.NewSelectList(list)
}

type textListParams struct {
	fieldParams
	Choices kernel.Choices
	Entries []string
}

// textList refuses an entry on the errors State finds on that entry alone, and
// badges it as its choice is badged.
func (s *session) textList(params textListParams) components.TextListModel {
	spec := s.fields[params.Index]
	return components.NewTextList(components.NewTextListParams{
		Title: titleOf(spec), Description: spec.Description, Entries: params.Entries,
		Required: s.fieldMessage(kernel.FieldError{Path: spec.Path, Code: kernel.CodeRequired}),
		Check: func(typed components.TextListEntry) (string, error) {
			entry := strings.TrimSpace(typed.Entry)
			entries := append(slices.Clone(typed.Entries), entry)
			return entry, s.check(checkParams{
				Index: params.Index, Request: params.Request, Value: kernel.List(entries),
				Entry: kernel.IndexPath(spec.Path, len(typed.Entries)),
			})
		},
		Badge: func(entry string) components.Badge {
			index := slices.IndexFunc(params.Choices.Options, func(option kernel.Option) bool { return option.Value == entry })
			if index < 0 || len(params.Choices.Options[index].Badges) == 0 {
				return components.Badge{}
			}
			return badgeOf(params.Choices.Options[index].Badges[0])
		},
	})
}

// options are the field's choices, else the values its Enum accepts; the
// pinned option comes first.
func options(choices kernel.Choices, accepted []string) []kernel.Option {
	if len(choices.Options) == 0 {
		found := make([]kernel.Option, 0, len(accepted))
		for _, value := range accepted {
			found = append(found, kernel.Option{Value: value, Label: value})
		}
		return found
	}
	found := slices.Clone(choices.Options)
	index := slices.IndexFunc(found, func(option kernel.Option) bool { return option.Value == choices.Pinned })
	if index > 0 {
		pinned := found[index]
		found = append([]kernel.Option{pinned}, slices.Delete(found, index, index+1)...)
	}
	return found
}

func (s *session) selectItems(choices kernel.Choices) []components.SelectItem {
	found := choices.Options
	items := make([]components.SelectItem, 0, len(found))
	for _, option := range found {
		item := components.SelectItem{Label: labelOfOption(option), Value: option.Value, Danger: option.Danger, Badges: badgesOf(option.Badges)}
		if option.Refusal != nil {
			item.Disabled = true
			item.Badges = append(item.Badges, components.Badge{Text: s.fieldMessage(*option.Refusal), Variant: components.BadgeWarning})
		}
		items = append(items, item)
	}
	return items
}

func (s *session) multiItems(choices kernel.Choices, selected []string) []components.MultiSelectItem {
	items := make([]components.MultiSelectItem, 0, len(choices.Options))
	for _, option := range choices.Options {
		badges := badgesOf(option.Badges)
		if option.Refusal != nil {
			badges = append(badges, components.Badge{Text: s.fieldMessage(*option.Refusal), Variant: components.BadgeWarning})
		}
		items = append(items, components.MultiSelectItem{Label: labelOfOption(option), Value: option.Value, Selected: slices.Contains(selected, option.Value), Badges: badges})
	}
	return items
}

// reorderItems keep the order the value gives, then whatever choice it lacks.
func reorderItems(choices kernel.Choices, ordered []string) []components.ReorderItem {
	labels := map[string]string{}
	values := slices.Clone(ordered)
	for _, option := range choices.Options {
		labels[option.Value] = labelOfOption(option)
		if !slices.Contains(values, option.Value) {
			values = append(values, option.Value)
		}
	}
	items := make([]components.ReorderItem, 0, len(values))
	for _, value := range values {
		label, known := labels[value]
		if !known {
			label = value
		}
		items = append(items, components.ReorderItem{Label: label, Value: value})
	}
	return items
}

// decisionRows are the keys of the value, each choosing among the options.
func decisionRows(choices kernel.Choices, value kernel.Value) []components.DecisionRow {
	decided, _ := value.(kernel.Decisions)
	values := make([]string, 0, len(choices.Options))
	for _, option := range choices.Options {
		values = append(values, option.Value)
	}
	rows := make([]components.DecisionRow, 0, len(decided))
	for _, key := range slices.Sorted(maps.Keys(decided)) {
		rows = append(rows, components.DecisionRow{Key: key, Options: values, Choice: decided[key]})
	}
	return rows
}

// valueOf reads a step's answer back as the value of its field.
func valueOf(model any) kernel.Value {
	switch answered := model.(type) {
	case components.TextInputModel:
		return kernel.Text(answered.Value())
	case components.ConfirmModel:
		return kernel.Bool(answered.Confirmed())
	case components.SelectListModel:
		return kernel.Text(answered.Value())
	case components.MultiSelectModel:
		return kernel.List(answered.Values())
	case components.ReorderListModel:
		return kernel.List(answered.Values())
	case components.TextListModel:
		return kernel.List(answered.Values())
	case components.DecisionListModel:
		return kernel.Decisions(answered.Values())
	}
	return nil
}

// summary is the answer in the trail, with where it came from when the user
// kept what a default offered.
func (s *session) summary(i int, model any) string {
	value := valueOf(model)
	decided := s.decided[i]
	kept := decided.origin == kernel.OriginDefault || decided.origin == kernel.OriginConfig
	if !kept || !reflect.DeepEqual(value, decided.offered) {
		return valueText(value)
	}
	return trailOf(valueText(value), string(decided.origin))
}

func valueText(value kernel.Value) string {
	switch shown := value.(type) {
	case kernel.Text:
		return string(shown)
	case kernel.Bool:
		if shown {
			return yesText
		}
		return noText
	case kernel.List:
		if len(shown) == 0 {
			return domain.SummaryNone
		}
		return strings.Join(shown, listSeparator)
	case kernel.Decisions:
		pairs := make([]string, 0, len(shown))
		for _, key := range slices.Sorted(maps.Keys(shown)) {
			pairs = append(pairs, fmt.Sprintf(pairFmt, key, shown[key]))
		}
		if len(pairs) == 0 {
			return domain.SummaryNone
		}
		return strings.Join(pairs, listSeparator)
	}
	return ""
}

func trailOf(value, origin string) string {
	return fmt.Sprintf(trailFmt, value, origin)
}

func textOf(value kernel.Value) string {
	shown, _ := value.(kernel.Text)
	return string(shown)
}

func list(value kernel.Value) []string {
	shown, _ := value.(kernel.List)
	return shown
}

func placeholder(spec kernel.FieldSpec) components.SelectListModel {
	return components.NewSelectList(components.NewSelectListParams{Title: titleOf(spec), Description: spec.Description})
}

func labelOf(spec kernel.FieldSpec) string {
	if spec.Label == "" {
		return spec.Path
	}
	return spec.Label
}

func titleOf(spec kernel.FieldSpec) string {
	if spec.Title == "" {
		return labelOf(spec)
	}
	return spec.Title
}

func labelOfOption(option kernel.Option) string {
	if option.Label == "" {
		return option.Value
	}
	return option.Label
}
