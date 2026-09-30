package main

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func at(file string) token.Position { return token.Position{Filename: file, Line: 1, Column: 1} }

func TestAMigratingEntryWithoutItsCountIsRefused(t *testing.T) {
	if _, err := parseMigrating("mutation internal/commands/wt/env\\.go\n"); err == nil {
		t.Error("an entry with no site count was accepted: the list could grow without anyone noticing")
	}
}

func TestAMigratingEntryFailsTheSiteBeyondItsCount(t *testing.T) {
	list, err := parseMigrating("# comment\nmutation internal/commands/wt/env\\.go 1\n")
	if err != nil {
		t.Fatal(err)
	}
	got := judge(judgeParams{Migrating: list, Findings: []finding{
		{pos: at("internal/commands/wt/env.go"), rule: "mutation", msg: "a"},
		{pos: at("internal/commands/wt/env.go"), rule: "mutation", msg: "b"},
	}})
	if !got.failed {
		t.Errorf("two sites under a count of one passed:\n%s", strings.Join(got.lines, "\n"))
	}
}

func TestAMigratingEntryCoveringFewerSitesSaysItCanShrink(t *testing.T) {
	list, err := parseMigrating("mutation internal/commands/wt/env\\.go 3\nmutation internal/commands/checkout/ 1\n")
	if err != nil {
		t.Fatal(err)
	}
	got := judge(judgeParams{Migrating: list, Findings: []finding{
		{pos: at("internal/commands/wt/env.go"), rule: "mutation", msg: "a"},
	}})
	if got.failed {
		t.Errorf("a site within its count failed:\n%s", strings.Join(got.lines, "\n"))
	}
	notes := strings.Join(got.notes, "\n")
	if !strings.Contains(notes, "lower it to 1") || !strings.Contains(notes, "remove it") {
		t.Errorf("notes = %q, want one entry to lower and one to remove", notes)
	}
}

func TestALegacyRuneFailsBeyondItsCount(t *testing.T) {
	findings := []finding{
		{pos: at("a.go"), rule: "fontcover", msg: "m", legacy: "▸"},
		{pos: at("b.go"), rule: "fontcover", msg: "m", legacy: "▸"},
	}
	got := judge(judgeParams{Findings: collapseLegacy(collapseLegacyParams{Findings: findings, Budgets: map[string]int{"▸": 1}})})
	if !got.failed {
		t.Errorf("two sites of a rune allowed once passed:\n%s", strings.Join(got.lines, "\n"))
	}
}

func TestALegacyRuneUnderItsCountSaysItCanShrink(t *testing.T) {
	findings := []finding{{pos: at("a.go"), rule: "fontcover", msg: "m", legacy: "▸"}}
	got := judge(judgeParams{Findings: collapseLegacy(collapseLegacyParams{Findings: findings, Budgets: map[string]int{"▸": 4, "⚠": 2}})})
	if got.failed {
		t.Errorf("a rune within its count failed:\n%s", strings.Join(got.lines, "\n"))
	}
	all := strings.Join(append(got.lines, got.notes...), "\n")
	if !strings.Contains(all, "lower it to 1") || !strings.Contains(all, `"⚠" has no site left`) {
		t.Errorf("report = %q, want ▸ lowered to 1 and ⚠ named as gone", all)
	}
}

func yesFlagFindings(t *testing.T, register string) []finding {
	t.Helper()
	src := `package cmd

func newCmd() *cobra.Command {
	cmd := &cobra.Command{}
	` + register + `
	return cmd
}

func run() { _ = shared.Interactive(nil) }
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "cmd.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	return checkYesFlag(fset, "cmd.go", file)
}

// --yes is the one spelling of the confirmation axis: the helper that also
// registered --non-interactive is gone, and must not satisfy the rule if it
// ever comes back.
func TestACommandReadingTheGateRegistersYes(t *testing.T) {
	if got := yesFlagFindings(t, `shared.AddYesFlag(cmd, "")`); len(got) != 0 {
		t.Errorf("findings = %v, want none for a command registering --yes", got)
	}
	if got := yesFlagFindings(t, `shared.AddNoPromptFlags(cmd, "")`); len(got) != 1 {
		t.Errorf("findings = %v, want the retired helper refused", got)
	}
	if got := yesFlagFindings(t, ``); len(got) != 1 {
		t.Errorf("findings = %v, want a command with no --yes refused", got)
	}
}
