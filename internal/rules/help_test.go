package rules_test

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func TestHelpSectionsAreOrderedRowMajor(t *testing.T) {
	sections := rules.HelpSections(true)
	titles := make([]string, 0, len(sections))
	for _, section := range sections {
		if len(section.Entries) == 0 {
			t.Fatalf("section %q has no entries", section.Title)
		}
		titles = append(titles, section.Title)
	}

	want := []string{
		domain.DashboardHelpSectionNav,
		domain.DashboardHelpSectionAct,
		domain.DashboardHelpSectionRun,
		domain.DashboardHelpSectionMouse,
		domain.DashboardHelpSectionView,
	}
	if len(titles) != len(want) {
		t.Fatalf("got %d sections %v, want %d", len(titles), titles, len(want))
	}
	for index, title := range want {
		if titles[index] != title {
			t.Errorf("section %d = %q, want %q", index, titles[index], title)
		}
	}
}

func TestHelpSectionsDocumentEveryDashboardKey(t *testing.T) {
	keys := ""
	for _, section := range rules.HelpSections(true) {
		for _, entry := range section.Entries {
			keys += entry.Keys + " "
		}
	}
	for _, key := range []string{
		domain.KeyNew, domain.KeyMenu, domain.KeyActions, domain.KeyOpenPR,
		domain.KeyFastForward, domain.KeyToggleOutput, domain.KeyRefresh,
		"j", "k", "g", "G", "pgup", "pgdown", "tab", "enter", "esc", "h", "l",
	} {
		if !containsField(keys, key) {
			t.Errorf("key %q is bound by the dashboard but missing from the reference", key)
		}
	}
}

func containsField(keys, key string) bool {
	for _, field := range splitFields(keys) {
		if field == key {
			return true
		}
	}
	return false
}

func splitFields(s string) []string {
	fields := []string{}
	current := ""
	for _, r := range s {
		if r == ' ' {
			if current != "" {
				fields = append(fields, current)
			}
			current = ""
			continue
		}
		current += string(r)
	}
	if current != "" {
		fields = append(fields, current)
	}
	return fields
}

func TestComputeHelpLayoutPairsSectionsWhenWide(t *testing.T) {
	layout := rules.ComputeHelpLayout(rules.HelpLayoutParams{
		Sections:     rules.HelpSections(true),
		ScreenWidth:  120,
		ScreenHeight: 40,
	})

	sections := len(rules.HelpSections(true))
	wantBands := (sections + 1) / 2
	if len(layout.Bands) != wantBands {
		t.Fatalf("got %d bands, want %d", len(layout.Bands), wantBands)
	}
	// An odd number of sections leaves the last band half empty; every other
	// band is a full pair.
	for index, band := range layout.Bands[:len(layout.Bands)-1] {
		if len(band) != 2 {
			t.Fatalf("band %d holds %d sections, want 2", index, len(band))
		}
	}
	if layout.Bands[0][0].Title != domain.DashboardHelpSectionNav ||
		layout.Bands[0][1].Title != domain.DashboardHelpSectionAct {
		t.Errorf("first band = %q|%q, want NAV|ACT", layout.Bands[0][0].Title, layout.Bands[0][1].Title)
	}
	if len(layout.KeyWidth) != 2 || len(layout.TextWidth) != 2 {
		t.Fatalf("got %d key widths and %d text widths, want 2 of each", len(layout.KeyWidth), len(layout.TextWidth))
	}
	if layout.Scrollable {
		t.Error("a 40-row screen holds the whole reference; it must not scroll")
	}
}

func TestComputeHelpLayoutStacksSectionsWhenNarrow(t *testing.T) {
	layout := rules.ComputeHelpLayout(rules.HelpLayoutParams{
		Sections:     rules.HelpSections(true),
		ScreenWidth:  60,
		ScreenHeight: 40,
	})

	if len(layout.Bands) != len(rules.HelpSections(true)) {
		t.Fatalf("got %d bands, want one per section", len(layout.Bands))
	}
	for index, band := range layout.Bands {
		if len(band) != 1 {
			t.Fatalf("band %d holds %d sections, want 1", index, len(band))
		}
	}
	if len(layout.KeyWidth) != 1 {
		t.Fatalf("got %d key widths, want 1", len(layout.KeyWidth))
	}
}

func TestComputeHelpLayoutSizesColumnsOnTheirOwnContent(t *testing.T) {
	sections := []domain.HelpSection{
		{Title: "A", Entries: []domain.HelpEntry{{Keys: "x", Text: "short"}}},
		{Title: "B", Entries: []domain.HelpEntry{{Keys: "long-key", Text: "a much longer description"}}},
	}
	layout := rules.ComputeHelpLayout(rules.HelpLayoutParams{
		Sections:     sections,
		ScreenWidth:  200,
		ScreenHeight: 40,
	})

	if layout.KeyWidth[0] >= layout.KeyWidth[1] {
		t.Errorf("key widths %v: a column must be sized on its own keys", layout.KeyWidth)
	}
	if layout.TextWidth[0] >= layout.TextWidth[1] {
		t.Errorf("text widths %v: a column must be sized on its own text", layout.TextWidth)
	}
}

func TestComputeHelpLayoutNeverExceedsTheScreen(t *testing.T) {
	for _, screen := range [][2]int{{120, 40}, {100, 24}, {80, 20}, {60, 30}, {40, 12}, {20, 8}} {
		layout := rules.ComputeHelpLayout(rules.HelpLayoutParams{
			Sections:     rules.HelpSections(true),
			ScreenWidth:  screen[0],
			ScreenHeight: screen[1],
		})
		if outer := layout.Inner + domain.DashboardHelpFrame; outer > screen[0] {
			t.Errorf("%dx%d: box is %d wide, screen is %d", screen[0], screen[1], outer, screen[0])
		}
		outerHeight := layout.BodyRows + domain.DashboardHelpChrome + domain.DashboardModalChrome
		if outerHeight > screen[1] {
			t.Errorf("%dx%d: box is %d tall, screen is %d", screen[0], screen[1], outerHeight, screen[1])
		}
	}
}

func TestComputeHelpLayoutScrollsWhenTheScreenIsShort(t *testing.T) {
	layout := rules.ComputeHelpLayout(rules.HelpLayoutParams{
		Sections:     rules.HelpSections(true),
		ScreenWidth:  120,
		ScreenHeight: 12,
	})

	if !layout.Scrollable {
		t.Fatal("a 12-row screen cannot hold the reference; it must scroll")
	}
	if layout.BodyRows >= layout.ContentRows {
		t.Errorf("BodyRows %d must be under ContentRows %d", layout.BodyRows, layout.ContentRows)
	}
}

func TestComputeHelpLayoutHandlesNoRoom(t *testing.T) {
	layout := rules.ComputeHelpLayout(rules.HelpLayoutParams{
		Sections:     rules.HelpSections(true),
		ScreenWidth:  0,
		ScreenHeight: 0,
	})
	if layout.Inner > 0 || layout.BodyRows > 0 {
		t.Errorf("no screen leaves no box, got inner %d rows %d", layout.Inner, layout.BodyRows)
	}
}

func TestHelpLeavesTheRunSectionOutWithoutARunModule(t *testing.T) {
	for _, section := range rules.HelpSections(false) {
		if section.Title == domain.DashboardHelpSectionRun {
			t.Fatalf("the %s section is listed for a project that runs no jobs", section.Title)
		}
	}
	if with, without := len(rules.HelpSections(true)), len(rules.HelpSections(false)); with != without+1 {
		t.Errorf("sections with a run module = %d, without = %d, want exactly the run section apart", with, without)
	}
}
