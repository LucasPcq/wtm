package cli

import (
	"cmp"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/command/configshow"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
	"github.com/LucasPcq/wtm/next/internal/ui/widget"
)

// drawShow draws config.toml as a document, a section at a time, each setting its value, its default, what it does and the problem it holds; then the remembered answers, then the read's other warnings.
func drawShow(w io.Writer, read configshow.Outcome, _ configshow.Request, view output.View) {
	if len(read.Items) == 1 && read.Items[0].Detail != nil {
		drawDocument(w, *read.Items[0].Detail, read.Warnings)
	}
	steps := output.WarningSteps(read.Warnings, view.CommandLine)
	read.Warnings = slices.DeleteFunc(slices.Clone(read.Warnings), func(warning domain.Warning) bool { return warning.Code == configshow.CodeInvalidValue })
	output.ReadEnd(w, read)
	if len(steps) > 0 {
		output.Blank(w)
		output.NextSteps(w, steps)
	}
}

func drawDocument(w io.Writer, d configshow.Detail, warnings []domain.Warning) {
	st := output.StylesOf(w)
	labels := map[string]string{}
	width := 0
	for _, question := range remembered {
		width = max(width, len(question))
	}
	for _, section := range sections {
		for _, path := range section.Paths {
			said, _ := form.Settings{}.Words(path)
			labels[path], width = said.Label, max(width, len(said.Label))
		}
	}
	// the value column is the same in every section, and the help wraps within the block
	pad := func(label string) string { return label + strings.Repeat(" ", width-len(label)) }
	wrap := func(text string) []string {
		return strings.Split(widget.Wrap(text, output.BlockWidth(w)-width-len(output.Indent)), "\n")
	}

	for i, section := range sections {
		if i > 0 {
			output.Blank(w)
		}
		output.SectionTitle(w, section.Title)
		var rows [][]string
		for _, path := range section.Paths {
			at := slices.IndexFunc(d.Settings.Fields, func(f domain.Field) bool { return f.Spec().Path == path })
			if at < 0 {
				continue
			}
			said, _ := form.Settings{}.Words(path)
			lines := valuesOf(d, d.Settings.Fields[at])
			rows = append(rows, []string{pad(labels[path]), lines[0]})
			for _, line := range lines[1:] {
				rows = append(rows, []string{"", line})
			}
			for _, warning := range warnings {
				if warning.Code == configshow.CodeInvalidValue && fieldOf(warning.Params["path"]) == path {
					rows = append(rows, []string{"", st.Warning.Render(output.GlyphAttention) + " " + warning.Message})
				}
			}
			for _, line := range wrap(said.Help) {
				rows = append(rows, []string{"", st.Muted.Render(line)})
			}
		}
		output.Columns(w, rows)
	}

	output.Blank(w)
	output.SectionTitle(w, rememberedTitle)
	if len(d.Remembered) == 0 {
		output.Message(w, st.Muted.Render("none: create asks each question"))
		return
	}
	var rows [][]string
	for _, answer := range d.Remembered {
		rows = append(rows, []string{pad(cmp.Or(remembered[answer.Question], answer.Question)), answer.Value})
		for _, warning := range warnings {
			if answer.Problem != nil && warning.Code == configshow.CodeInvalidValue && warning.Params["path"] == answer.Problem.Path {
				rows = append(rows, []string{"", st.Warning.Render(output.GlyphAttention) + " " + warning.Message})
			}
		}
	}
	output.Columns(w, rows)
}

// fieldOf is the setting a problem's path names: env_files of env_files[1].
func fieldOf(path string) string {
	if at := strings.IndexAny(path, "[."); at >= 0 {
		return path[:at]
	}
	return path
}

// valuesOf are the lines of a setting's value: one for a value, with `· default` when it is the default and the values it takes; one per hook; the files on one line.
func valuesOf(d configshow.Detail, field domain.Field) []string {
	switch f := field.(type) {
	case domain.TextField:
		return []string{f.Value + defaultMark(d, f.Path)}
	case domain.SelectField:
		value := cmp.Or(f.Value, "none") + defaultMark(d, f.Path)
		if !f.Closed {
			return []string{value}
		}
		taken := make([]string, 0, len(f.Options))
		for _, option := range f.Options {
			taken = append(taken, option.Value)
		}
		return []string{value, "takes " + strings.Join(taken, ", ")}
	case domain.MultiSelectField:
		if len(f.Picked) == 0 {
			return []string{"none"}
		}
		return []string{strings.Join(f.Picked, ", ")}
	case domain.HooksField:
		if len(f.Entries) == 0 {
			return []string{"none"}
		}
		lines := make([]string, 0, len(f.Entries))
		for _, hook := range f.Entries {
			lines = append(lines, hookLine(hook))
		}
		return lines
	case domain.TextListField:
		return []string{strings.Join(f.Entries, ", ")}
	case domain.ConfirmField:
		return []string{strconv.FormatBool(f.Value)}
	}

	return []string{""}
}

func hookLine(hook project.Hook) string {
	line := hook.Cmd
	if hook.Cwd != "" {
		line += " · in " + hook.Cwd
	}
	if hook.ContinueOnError {
		line += " · goes on if it fails"
	}
	return line
}

// defaultMark is ` · default` when the setting at path holds its default.
func defaultMark(d configshow.Detail, path string) string {
	if slices.ContainsFunc(d.Defaults, func(def configshow.Default) bool { return def.Path == path && def.InUse }) {
		return " · default"
	}
	return ""
}
