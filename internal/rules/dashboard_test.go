package rules

import (
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

// TestDashboardHeaderHeightDegradesBelowTheThreshold pins the header's own
// height rule: the six-row signature block above
// domain.DashboardHeaderTallThreshold rows, the compact three-row header
// below it — this is what makes the height terminal-dependent instead of a
// constant read directly.
func TestDashboardHeaderHeightDegradesBelowTheThreshold(t *testing.T) {
	if got := DashboardHeaderHeight(domain.DashboardHeaderTallThreshold); got != domain.DashboardHeaderTallHeight {
		t.Errorf("DashboardHeaderHeight(%d) = %d, want the tall height %d",
			domain.DashboardHeaderTallThreshold, got, domain.DashboardHeaderTallHeight)
	}
	if got := DashboardHeaderHeight(domain.DashboardHeaderTallThreshold - 1); got != domain.DashboardHeaderCompactHeight {
		t.Errorf("DashboardHeaderHeight(%d) = %d, want the compact height %d",
			domain.DashboardHeaderTallThreshold-1, got, domain.DashboardHeaderCompactHeight)
	}
}

// TestComputeDashboardLayoutMarksTheTallHeader pins that the layout, not the
// renderer, decides whether the tall header shows — a magic-threshold
// comparison re-derived in tui/ is exactly the decision logic that belongs
// here instead.
func TestComputeDashboardLayoutMarksTheTallHeader(t *testing.T) {
	tall := ComputeDashboardLayout(DashboardLayoutParams{Width: 120, Height: 40})
	if !tall.HeaderTall {
		t.Error("a 40-row terminal must get the tall header")
	}
	if tall.Tabs.Height != domain.DashboardHeaderTallHeight {
		t.Errorf("Tabs.Height = %d, want the tall height %d", tall.Tabs.Height, domain.DashboardHeaderTallHeight)
	}

	compact := ComputeDashboardLayout(DashboardLayoutParams{Width: 120, Height: 24})
	if compact.HeaderTall {
		t.Error("a 24-row terminal must fall back to the compact header")
	}
	if compact.Tabs.Height != domain.DashboardHeaderCompactHeight {
		t.Errorf("Tabs.Height = %d, want the compact height %d", compact.Tabs.Height, domain.DashboardHeaderCompactHeight)
	}
}

func TestComputeDashboardLayoutSplitsWideTerminals(t *testing.T) {
	layout := ComputeDashboardLayout(DashboardLayoutParams{Width: 120, Height: 40})

	if layout.Narrow {
		t.Fatal("120 columns is above the narrow threshold")
	}
	if !layout.ListVisible || !layout.DetailVisible {
		t.Fatalf("both panels should show, got list=%v detail=%v", layout.ListVisible, layout.DetailVisible)
	}
	if layout.List.Width+layout.Detail.Width != 120 {
		t.Errorf("panels span %d columns, want the full 120", layout.List.Width+layout.Detail.Width)
	}
	if layout.Detail.X != layout.List.Width {
		t.Errorf("detail starts at x=%d, want it flush against the list (%d)", layout.Detail.X, layout.List.Width)
	}
	if layout.List.Y != layout.Tabs.Height {
		t.Errorf("body starts at y=%d, want it right under the tab bar", layout.List.Y)
	}
}

func TestComputeDashboardLayoutNarrowDropsTheDetail(t *testing.T) {
	closed := ComputeDashboardLayout(DashboardLayoutParams{Width: 80, Height: 30})
	if !closed.Narrow || closed.DetailVisible || !closed.ListVisible {
		t.Fatalf("narrow closed = %+v, want list only", closed)
	}
	if closed.List.Width != 80 {
		t.Errorf("list width = %d, want the full width", closed.List.Width)
	}

	opened := ComputeDashboardLayout(DashboardLayoutParams{Width: 80, Height: 30, DetailOpen: true})
	if !opened.DetailVisible || opened.ListVisible {
		t.Fatalf("narrow opened = %+v, want detail only", opened)
	}
	if opened.Detail.Width != 80 {
		t.Errorf("detail width = %d, want the full width", opened.Detail.Width)
	}
}

func TestComputeDashboardLayoutNarrowThresholdIsExclusive(t *testing.T) {
	if ComputeDashboardLayout(DashboardLayoutParams{Width: domain.DashboardNarrowWidth, Height: 30}).Narrow {
		t.Error("exactly the threshold width must stay wide")
	}
	if !ComputeDashboardLayout(DashboardLayoutParams{Width: domain.DashboardNarrowWidth - 1, Height: 30}).Narrow {
		t.Error("one column under the threshold must turn narrow")
	}
}

func TestComputeDashboardLayoutOutputPanelFoldsAndUnfolds(t *testing.T) {
	folded := ComputeDashboardLayout(DashboardLayoutParams{Width: 120, Height: 40})
	expanded := ComputeDashboardLayout(DashboardLayoutParams{Width: 120, Height: 40, OutputExpanded: true})

	if folded.OutputLines != 0 {
		t.Errorf("folded output shows %d lines, want none", folded.OutputLines)
	}
	if folded.Output.Height != domain.DashboardChromeHeight {
		t.Errorf("folded output height = %d, want chrome only", folded.Output.Height)
	}
	if expanded.OutputLines != domain.DashboardOutputBodyHeight {
		t.Errorf("expanded output shows %d lines, want %d", expanded.OutputLines, domain.DashboardOutputBodyHeight)
	}
	if expanded.List.Height >= folded.List.Height {
		t.Error("unfolding the output panel must take its rows from the body")
	}
}

func TestComputeDashboardLayoutStacksWithoutOverlapOrGap(t *testing.T) {
	for _, expanded := range []bool{false, true} {
		layout := ComputeDashboardLayout(DashboardLayoutParams{Width: 120, Height: 40, OutputExpanded: expanded})
		if got := layout.List.Y + layout.List.Height; got != layout.Output.Y {
			t.Errorf("expanded=%v: body ends at y=%d, output starts at y=%d", expanded, got, layout.Output.Y)
		}
		if got := layout.Output.Y + layout.Output.Height; got != layout.Help.Y {
			t.Errorf("expanded=%v: output ends at y=%d, help sits at y=%d", expanded, got, layout.Help.Y)
		}
		if layout.Help.Y+layout.Help.Height != 40 {
			t.Errorf("expanded=%v: last row is %d, want 40", expanded, layout.Help.Y+layout.Help.Height)
		}
	}
}

func TestComputeDashboardLayoutShrinksTheOutputPanelBeforeTheBody(t *testing.T) {
	layout := ComputeDashboardLayout(DashboardLayoutParams{Width: 120, Height: 8, OutputExpanded: true})

	if layout.List.Height < minDashboardBody {
		t.Errorf("body height = %d, want at least %d", layout.List.Height, minDashboardBody)
	}
	if layout.Output.Height >= domain.DashboardChromeHeight+domain.DashboardOutputBodyHeight {
		t.Errorf("output height = %d, want it given up to the body", layout.Output.Height)
	}
}

func TestComputeDashboardLayoutSurvivesDegenerateSizes(t *testing.T) {
	for _, size := range [][2]int{
		{0, 0}, {1, 1}, {10, 2}, {200, 1}, {200, 2}, {200, 3},
		// Around the tall-header threshold: the six-row header must degrade
		// the same way the three-row one already did, at every size on
		// either side of the cutover.
		{200, 29}, {200, 30}, {200, 31}, {200, 6}, {200, 7}, {0, 30}, {1, 30},
	} {
		layout := ComputeDashboardLayout(DashboardLayoutParams{Width: size[0], Height: size[1]})
		for name, rect := range map[string]domain.Rect{
			"tabs": layout.Tabs, "list": layout.List, "detail": layout.Detail,
			"output": layout.Output, "help": layout.Help,
		} {
			if rect.Width < 0 || rect.Height < 0 || rect.X < 0 || rect.Y < 0 {
				t.Errorf("%dx%d: %s = %+v has a negative dimension", size[0], size[1], name, rect)
			}
		}
		if layout.ListRows < 0 || layout.OutputLines < 0 {
			t.Errorf("%dx%d: negative row counts", size[0], size[1])
		}

		// The header and the help bar must give way to each other on a
		// terminal too short for both at full size, rather than both
		// claiming their usual rows and overflowing it between them.
		body := max(layout.List.Height, layout.Detail.Height)
		if reserved := layout.Tabs.Height + body + layout.Output.Height + layout.Help.Height; reserved > size[1] {
			t.Errorf("%dx%d: rows reserved (tabs=%d body=%d output=%d help=%d = %d) exceed the terminal height",
				size[0], size[1], layout.Tabs.Height, body, layout.Output.Height, layout.Help.Height, reserved)
		}
	}
}

func TestDashboardScrollOffsetKeepsTheCursorVisible(t *testing.T) {
	tests := []struct {
		name   string
		params DashboardScrollParams
		want   int
	}{
		{"everything fits", DashboardScrollParams{Cursor: 3, Total: 5, Visible: 10}, 0},
		{"no room", DashboardScrollParams{Cursor: 3, Total: 5, Visible: 0}, 0},
		{"cursor above the window", DashboardScrollParams{Cursor: 2, Total: 20, Visible: 5, Offset: 7}, 2},
		{"cursor below the window", DashboardScrollParams{Cursor: 12, Total: 20, Visible: 5, Offset: 0}, 8},
		{"cursor inside the window", DashboardScrollParams{Cursor: 9, Total: 20, Visible: 5, Offset: 7}, 7},
		{"offset past the end", DashboardScrollParams{Cursor: 19, Total: 20, Visible: 5, Offset: 99}, 15},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := DashboardScrollOffset(tc.params); got != tc.want {
				t.Errorf("offset = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestClampIndex(t *testing.T) {
	tests := []struct{ index, count, want int }{
		{-1, 5, 0}, {0, 5, 0}, {4, 5, 4}, {9, 5, 4}, {3, 0, 0},
	}
	for _, tc := range tests {
		if got := ClampIndex(tc.index, tc.count); got != tc.want {
			t.Errorf("ClampIndex(%d, %d) = %d, want %d", tc.index, tc.count, got, tc.want)
		}
	}
}

func TestDashboardClampOffsetKeepsAFreeScrollInRange(t *testing.T) {
	tests := []struct {
		name   string
		params DashboardOffsetParams
		want   int
	}{
		{"everything fits", DashboardOffsetParams{Offset: 4, Total: 5, Visible: 10}, 0},
		{"no room", DashboardOffsetParams{Offset: 4, Total: 50, Visible: 0}, 0},
		{"negative", DashboardOffsetParams{Offset: -3, Total: 50, Visible: 10}, 0},
		{"in range", DashboardOffsetParams{Offset: 7, Total: 50, Visible: 10}, 7},
		{"past the end", DashboardOffsetParams{Offset: 999, Total: 50, Visible: 10}, 40},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := DashboardClampOffset(tc.params); got != tc.want {
				t.Errorf("offset = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestDashboardScrollOffsetToleratesAnOutOfRangeCursor(t *testing.T) {
	got := DashboardScrollOffset(DashboardScrollParams{Cursor: 99, Total: 10, Visible: 5, Offset: 99})

	if want := 5; got != want {
		t.Errorf("offset = %d, want %d — a stale cursor must not scroll past the end", got, want)
	}
}

func TestComputeMenuRectHangsUnderItsAnchor(t *testing.T) {
	rect := ComputeMenuRect(MenuRectParams{
		AnchorX: 10, AnchorY: 5,
		Width: 20, Height: 4,
		ScreenWidth: 120, ScreenHeight: 40,
	})

	if rect.X != 10 || rect.Y != 6 {
		t.Errorf("menu at (%d,%d), want it just under the cell it was opened from", rect.X, rect.Y)
	}
}

func TestComputeMenuRectFlipsAboveTheAnchorAtTheBottom(t *testing.T) {
	rect := ComputeMenuRect(MenuRectParams{
		AnchorX: 4, AnchorY: 38,
		Width: 20, Height: 4,
		ScreenWidth: 120, ScreenHeight: 40,
	})

	if rect.Y+rect.Height > 40 {
		t.Errorf("menu runs to y=%d, past the screen", rect.Y+rect.Height)
	}
	if rect.Y >= 38 {
		t.Errorf("menu at y=%d, want it flipped above the anchor", rect.Y)
	}
}

func TestComputeMenuRectStaysInsideTheRightEdge(t *testing.T) {
	rect := ComputeMenuRect(MenuRectParams{
		AnchorX: 115, AnchorY: 2,
		Width: 20, Height: 4,
		ScreenWidth: 120, ScreenHeight: 40,
	})

	if rect.X+rect.Width > 120 {
		t.Errorf("menu spans to x=%d, past the right edge", rect.X+rect.Width)
	}
}

func TestComputeMenuRectSurvivesAScreenSmallerThanTheMenu(t *testing.T) {
	rect := ComputeMenuRect(MenuRectParams{
		AnchorX: 2, AnchorY: 2,
		Width: 40, Height: 10,
		ScreenWidth: 20, ScreenHeight: 6,
	})

	if rect.X < 0 || rect.Y < 0 || rect.X+rect.Width > 20 || rect.Y+rect.Height > 6 {
		t.Errorf("menu = %+v, want it clamped inside a 20x6 screen", rect)
	}
}

func TestSplitRecapFieldSeparatesLabelFromValue(t *testing.T) {
	label, value, ok := SplitRecapField("Parent:      feat → main")
	if !ok {
		t.Fatal("a recap field must split")
	}
	if label != "Parent:      " {
		t.Errorf("label = %q, want the label and the run of spaces after it", label)
	}
	if value != "feat → main" {
		t.Errorf("value = %q", value)
	}
}

// A heading is not a field: it must be left exactly as the flow wrote it.
func TestSplitRecapFieldLeavesHeadingsAndProseWhole(t *testing.T) {
	for _, line := range []string{
		"Will delete:",
		"dev-a is currently rebased onto feat",
		"",
		":no label",
		"Label:no gap",
	} {
		if _, _, ok := SplitRecapField(line); ok {
			t.Errorf("%q must not read as a field", line)
		}
	}
}

func TestActiveWorktree(t *testing.T) {
	statuses := []domain.WorktreeStatus{
		{Branch: "main", Path: "/repo"},
		{Branch: "feat/ui", Path: "/repo.worktrees/feat-ui"},
		{Branch: "feat/ui-extra", Path: "/repo.worktrees/feat-ui-extra"},
		{Branch: "nested", Path: "/repo.worktrees/feat-ui/nested"},
	}
	cases := []struct {
		name string
		cwd  string
		want string
	}{
		{"racine du worktree", "/repo.worktrees/feat-ui", "feat/ui"},
		{"sous-dossier", "/repo.worktrees/feat-ui/internal/tui", "feat/ui"},
		{"préfixe voisin non confondu", "/repo.worktrees/feat-ui-extra", "feat/ui-extra"},
		{"worktree principal", "/repo/internal", "main"},
		{"le plus profond gagne", "/repo.worktrees/feat-ui/nested/pkg", "nested"},
		{"hors de tout worktree", "/ailleurs", ""},
		{"cwd vide", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ActiveWorktree(ActiveWorktreeParams{Cwd: c.cwd, Statuses: statuses})
			if got != c.want {
				t.Errorf("ActiveWorktree(%q) = %q, want %q", c.cwd, got, c.want)
			}
		})
	}
}

func TestAnimationsEnabledByDefault(t *testing.T) {
	if !AnimationsEnabled(domain.Config{}) {
		t.Error("les animations sont actives par défaut : une clé absente n'est pas un refus")
	}
}

func TestAnimationsCanBeDisabled(t *testing.T) {
	off := false
	cfg := domain.Config{Global: domain.GlobalConfig{UI: domain.UIConfig{Animations: &off}}}
	if AnimationsEnabled(cfg) {
		t.Error("ui.animations = false doit tout éteindre")
	}
}

func TestNoAnimationExceedsTheCap(t *testing.T) {
	durations := map[string]time.Duration{
		"glissement d'onglet": domain.DashboardTabSlide,
		"fondu de ligne":      domain.DashboardRowFlash,
	}
	for name, got := range durations {
		if got > domain.DashboardAnimationCap {
			t.Errorf("%s dure %v, plafond %v", name, got, domain.DashboardAnimationCap)
		}
	}
}

func TestTabSlideStartInterpolatesThenSettles(t *testing.T) {
	since := time.Now()

	if got := TabSlideStart(TabSlideParams{From: 0, To: 40, Since: since, Now: since, Duration: domain.DashboardTabSlide}); got != 0 {
		t.Errorf("at t=0, got %d, want From (0)", got)
	}
	mid := TabSlideStart(TabSlideParams{From: 0, To: 40, Since: since, Now: since.Add(domain.DashboardTabSlide / 2), Duration: domain.DashboardTabSlide})
	if mid <= 0 || mid >= 40 {
		t.Errorf("mid-slide, got %d, want strictly between From and To", mid)
	}
	if got := TabSlideStart(TabSlideParams{From: 0, To: 40, Since: since, Now: since.Add(domain.DashboardTabSlide), Duration: domain.DashboardTabSlide}); got != 40 {
		t.Errorf("once the duration has elapsed, got %d, want To (40)", got)
	}
	if got := TabSlideStart(TabSlideParams{From: 0, To: 40, Duration: domain.DashboardTabSlide}); got != 40 {
		t.Errorf("a zero Since means no slide in progress, got %d, want To (40)", got)
	}
}

func TestFlashLitFadesWithinItsDuration(t *testing.T) {
	since := time.Now()
	duration := domain.DashboardRowFlash

	if !FlashLit(FlashParams{Since: since, Now: since, Duration: duration}) {
		t.Error("a flash just triggered must be lit")
	}
	if FlashLit(FlashParams{Since: since, Now: since.Add(duration), Duration: duration}) {
		t.Error("a flash whose full duration has elapsed must no longer be lit")
	}
	if FlashLit(FlashParams{Since: time.Time{}, Now: since, Duration: duration}) {
		t.Error("no flash in progress (zero Since) must never read as lit")
	}
}

func TestFullBodyGivesTheWholeWidthToTheMainPanel(t *testing.T) {
	layout := ComputeDashboardLayout(DashboardLayoutParams{Width: 160, Height: 40, FullBody: true})

	if layout.DetailVisible {
		t.Error("DetailVisible = true, want the main panel alone")
	}
	if layout.List.Width != 160 {
		t.Errorf("List.Width = %d, want the full 160: a url does not read in 48 columns", layout.List.Width)
	}
	if !layout.ListVisible {
		t.Error("ListVisible = false, want the main panel drawn")
	}
}

func TestFullBodyStillReservesTheOutputPanelAndHelpBar(t *testing.T) {
	full := ComputeDashboardLayout(DashboardLayoutParams{Width: 160, Height: 40, FullBody: true})
	split := ComputeDashboardLayout(DashboardLayoutParams{Width: 160, Height: 40})

	if full.Output != split.Output {
		t.Errorf("Output = %+v, want it unchanged: only the body's split differs", full.Output)
	}
	if full.Help != split.Help {
		t.Errorf("Help = %+v, want it unchanged", full.Help)
	}
}
