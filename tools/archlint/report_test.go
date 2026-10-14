package main

import (
	"go/token"
	"strings"
	"testing"
)

func at(file string) token.Position { return token.Position{Filename: file, Line: 1, Column: 1} }

func TestAMigratingEntryWithoutItsCountIsRefused(t *testing.T) {
	if _, err := parseMigrating("mutation internal/surface/cli/wt/env\\.go\n"); err == nil {
		t.Error("an entry with no site count was accepted: the list could grow without anyone noticing")
	}
}

func TestAMigratingEntryFailsTheSiteBeyondItsCount(t *testing.T) {
	list, err := parseMigrating("# comment\nmutation internal/surface/cli/wt/env\\.go 1\n")
	if err != nil {
		t.Fatal(err)
	}
	got := judge(judgeParams{Migrating: list, Findings: []finding{
		{pos: at("internal/surface/cli/wt/env.go"), rule: "mutation", msg: "a"},
		{pos: at("internal/surface/cli/wt/env.go"), rule: "mutation", msg: "b"},
	}})
	if !got.failed {
		t.Errorf("two sites under a count of one passed:\n%s", strings.Join(got.lines, "\n"))
	}
}

func TestATrackedSiteIsCountedAndListedOnlyOnRequest(t *testing.T) {
	list, err := parseMigrating("mutation internal/surface/cli/wt/env\\.go 2\n")
	if err != nil {
		t.Fatal(err)
	}
	findings := []finding{
		{pos: at("internal/surface/cli/wt/env.go"), rule: "mutation", msg: "a"},
		{pos: at("internal/surface/cli/wt/env.go"), rule: "mutation", msg: "b"},
	}
	quiet := judge(judgeParams{Migrating: list, Findings: findings})
	if quiet.failed || quiet.tracked != 2 || len(quiet.lines) != 0 {
		t.Errorf("quiet run: failed %v, tracked %d, lines %q; want 2 sites counted and none printed", quiet.failed, quiet.tracked, quiet.lines)
	}
	listed := judge(judgeParams{Migrating: list, Findings: findings, Tracked: true})
	if len(listed.lines) != 2 || !strings.Contains(listed.lines[0], tagMigrating) {
		t.Errorf("tracked run: lines %q, want both sites tagged migrating", listed.lines)
	}
}

func TestAMigratingEntryCoveringFewerSitesSaysItCanShrink(t *testing.T) {
	list, err := parseMigrating("mutation internal/surface/cli/wt/env\\.go 3\nmutation internal/surface/cli/checkout/ 1\n")
	if err != nil {
		t.Fatal(err)
	}
	got := judge(judgeParams{Migrating: list, Findings: []finding{
		{pos: at("internal/surface/cli/wt/env.go"), rule: "mutation", msg: "a"},
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
