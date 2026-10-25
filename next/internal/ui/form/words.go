package form

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/ui/widget"
)

// Words are how the command line words one field of a command's form, which the core gives as data: the argument or flag that sets it (<branches>, --from), its name in the breadcrumb, the trail and its problems, what its answer decides (the wizard's description and the flag's usage), the problem of a multi-select answered with none, and how its options read, by value (a confirm's by true and false).
type Words struct {
	Flag    string
	Label   string
	Help    string
	Empty   string
	Options map[string]OptionWords
}

// OptionWords are how an option reads in its list (Label, else as the facts of the option word it) and how the trail names it once picked (Short, else by its value).
type OptionWords struct {
	Label string
	Short string
}

// A Worder words a field of a command's form on the questions that gave it; the zero questions word the schema, whose words are the flags and their usages.
type Worder[Q any] func(q Q, field domain.Field) Words

// A worded field is a field of a form with its words.
type worded struct {
	domain.Field
	Words
}

// wordedSchema is the schema of a command, the form of its zero questions, each field with its words.
func wordedSchema[Q Questions](words Worder[Q]) []worded {
	var zero Q
	return wordedForm(zero, words)
}

func wordedForm[Q Questions](q Q, words Worder[Q]) []worded {
	fields := q.Form().Fields
	found := make([]worded, 0, len(fields))
	for _, field := range fields {
		found = append(found, worded{Field: field, Words: words(q, field)})
	}

	return found
}

// flagsOf spells each field path of schema as the argument or flag that sets it, with the usage of that flag, its help; the first field answering a path gives both. A schema with a rememberable field also has --remember.
func flagsOf(schema []worded) (flags, usages map[string]string) {
	flags, usages = map[string]string{}, map[string]string{}
	var rememberable []string
	for _, field := range schema {
		path := field.Spec().Path
		if _, seen := flags[path]; !seen {
			flags[path], usages[path] = field.Flag, field.Help
		}
		if remembers(field.Field) {
			rememberable = append(rememberable, path)
		}
	}

	if len(rememberable) > 0 {
		flags[domain.PathRemember] = "--" + domain.PathRemember
		usages[domain.PathRemember] = "The answers to always use in this repository: " + strings.Join(rememberable, ", ") + " (repeatable)"
	}

	return flags, usages
}

func remembers(field domain.Field) bool {
	switch field := field.(type) {
	case domain.SelectField:
		return field.Rememberable
	case domain.ConfirmField:
		return field.Rememberable
	case domain.MultiSelectField, domain.TextField, domain.TextListField:
	}

	return false
}

// labelOf spells a field path with the label of the first field of schema that answers it, keeping an entry's index: branches[1] reads Branches[1].
func labelOf(schema []worded, path string) string {
	name, index, indexed := strings.Cut(path, "[")
	at := slices.IndexFunc(schema, func(f worded) bool { return f.Spec().Path == name })
	switch {
	case at < 0:
		return path
	case indexed:
		return schema[at].Label + "[" + index
	}

	return schema[at].Label
}

// ----------------------------------------------------------------------------
// The facts of an option

// tagWords are how the command line words the tag of an option.
var tagWords = map[domain.OptionTag]string{
	domain.TagLocal:      "existing",
	domain.TagCheckedOut: "worktree exists",
	domain.TagRemote:     "on origin",
	domain.TagCurrent:    "current",
	domain.TagMain:       "main",
}

// Options are the rows of a select list that offer options, each read as its words say, else as its facts word it.
func Options(found []domain.Option, words Words) []widget.Option {
	options := make([]widget.Option, 0, len(found))
	for _, o := range found {
		options = append(options, widget.Option{Label: optionLabel(o, words), Value: o.Value, Badge: badgeOf(o)})
	}

	return options
}

// optionLabel is how option reads in its list: as words says, else its value, marked when the facts call for it.
func optionLabel(option domain.Option, words Words) string {
	if said := words.Options[option.Value].Label; said != "" {
		return said
	}

	switch {
	case option.Change != nil:
		return option.Change.Label()
	case option.Base:
		return option.Value + " (default)"
	case option.Detached:
		return option.Value + " (detached)"
	}

	return option.Value
}

// badgeOf is the badge the wizard shows beside option: what its tag says, the status of its change, how many changes it holds, or how far it stands from origin.
func badgeOf(option domain.Option) string {
	switch {
	case option.Tag != "":
		return tagWords[option.Tag]
	case option.Change != nil:
		return option.Change.Status.Tag()
	case option.Changes > 0:
		return counted(option.Changes, "change")
	case option.Origin != nil:
		return originBadge(*option.Origin)
	}

	return ""
}

func originBadge(origin worktree.Origin) string {
	switch origin.State {
	case worktree.Behind:
		return strconv.Itoa(origin.Behind) + " behind"
	case worktree.Ahead:
		return strconv.Itoa(origin.Ahead) + " ahead"
	case worktree.Diverged:
		return strconv.Itoa(origin.Ahead) + " ahead, " + strconv.Itoa(origin.Behind) + " behind"
	case worktree.NoOrigin, worktree.UpToDate:
	}

	return ""
}

func counted(count int, noun string) string {
	if count == 1 {
		return "1 " + noun
	}

	return strconv.Itoa(count) + " " + noun + "s"
}

// shortOf names value as the trail does: the Short its words give it, else the value itself.
func shortOf(words Words, value string) string {
	return cmp.Or(words.Options[value].Short, value)
}
