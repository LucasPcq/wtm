package dashboard

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
)

func detailModel(t *testing.T, status domain.WorktreeStatus) Model {
	t.Helper()
	model := New(RunParams{})
	t.Cleanup(model.Close)
	model = update(model, tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
	return update(model, worktreesMsg{statuses: []domain.WorktreeStatus{status}, parents: map[string]string{}})
}

func lineContaining(t *testing.T, lines []string, needle string) string {
	t.Helper()
	for _, line := range lines {
		if strings.Contains(line, needle) {
			return line
		}
	}
	t.Fatalf("no line contains %q", needle)
	return ""
}

// detailLinksRow finds the row index carrying the LINKS section's title, the
// signal state 3 (§8) must not move when the placeholder it reserves is
// replaced by the sections that actually land.
func detailLinksRow(t *testing.T, view string) int {
	t.Helper()
	lines := strings.Split(view, "\n")
	for index, line := range lines {
		if strings.Contains(line, domain.DetailSectionLinks) {
			return index
		}
	}
	t.Fatal("the rendered view has no LINKS section")
	return -1
}

// The list and detail panels sit side by side, so a raw model.View() line
// mixes both panels' content on one terminal row — asserting on the detail
// panel's own lines is what actually isolates its title row.
func TestDetailHeaderCarriesIdentityNotState(t *testing.T) {
	model := detailModel(t, domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x", IsDirty: true})
	lines := model.detailBody(model.layout())

	titleLine := lineContaining(t, lines, "feat/x")
	if strings.Contains(titleLine, "dirty") {
		t.Error("the state pill belongs to the vital strip, not the title line")
	}
}

func TestDetailOmitsEmptyFields(t *testing.T) {
	model := detailModel(t, domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"})
	view := model.View()

	for _, dead := range []string{"PR       none", "Parent   —", "up to date"} {
		if strings.Contains(view, dead) {
			t.Errorf("the panel must not recite %q anymore", dead)
		}
	}
}

// TestVitalStripWrapsChipByChip exercises vitalStripLines directly rather
// than through model.View(): at narrowWide the layout gives the detail panel
// a zero-width rect (it is below domain.DashboardNarrowWidth and the detail
// is not open), so detailBody returns nil and a View()-based assertion would
// pass on an empty render — proving nothing.
func TestVitalStripWrapsChipByChip(t *testing.T) {
	chips := rules.VitalChips(rules.VitalChipsParams{
		Status: domain.WorktreeStatus{
			Branch: "feat/x", Path: "/wt/x", IsDirty: true, CommitsAhead: 3,
			OriginAhead: 2, OriginBehind: 1, OriginState: domain.DivergenceDiverged,
		},
	})
	if len(chips) < 3 {
		t.Fatalf("chips = %v, want at least 3 to make a wrap meaningful", chips)
	}

	// Wide enough for the first chip plus its separator, not for the second:
	// forces an actual wrap rather than merely fitting everything on one line.
	width := lipgloss.Width(chips[0].Text) + lipgloss.Width(domain.DashboardMetaSeparator) + 1

	lines := vitalStripLines(chips, width, false)
	if len(lines) < 2 {
		t.Fatalf("vitalStripLines at width %d = %d line(s), want at least 2 — this proves nothing if it never wraps", width, len(lines))
	}

	joined := stripANSI(strings.Join(lines, "\n"))
	for _, chip := range chips {
		if !strings.Contains(joined, chip.Text) {
			t.Errorf("chip %q missing from the wrapped strip %q — a chip is never cut mid-way", chip.Text, joined)
		}
	}
}

func TestFirstLoadReservesHeightInsteadOfJumping(t *testing.T) {
	model := detailModel(t, domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"})
	model.detailLoading = "feat/x"
	model.detailSince = time.Now().Add(-time.Second)

	before := detailLinksRow(t, model.View())
	model = update(model, detailMsg{branch: "feat/x", detail: domain.WorktreeDetail{
		Branch:  "feat/x",
		Commits: []domain.CommitSummary{{SHA: "abc1234", Subject: "feat: x"}},
	}})
	after := detailLinksRow(t, model.View())

	if before != after {
		t.Errorf("LINKS moved from row %d to row %d — the placeholder must reserve its height", before, after)
	}
	if !strings.Contains(model.View(), "abc1234") {
		t.Error("the landed data must replace the placeholder")
	}
}

func TestLegitimateAbsenceIsNotAFailure(t *testing.T) {
	model := detailModel(t, domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"})
	model = update(model, detailMsg{branch: "feat/x", detail: domain.WorktreeDetail{
		Branch:   "feat/x",
		EnvDrift: domain.EnvDriftSummary{Configured: false},
	}})

	view := model.View()
	if !strings.Contains(view, domain.DashboardNotConfigured) {
		t.Error("no env file configured is a legitimate absence, not a success to fake")
	}
	if strings.Contains(view, "unavailable") {
		t.Error("a legitimate absence must not read as a failure")
	}
	notConfiguredLine := lineContaining(t, strings.Split(view, "\n"), domain.DashboardNotConfigured)
	if strings.Contains(notConfiguredLine, "⚠") {
		t.Error("a legitimate absence stays glyph-free — that contrast is what tells the two apart")
	}
}

func TestFailureSaysWhy(t *testing.T) {
	model := detailModel(t, domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"})
	model = update(model, detailMsg{branch: "feat/x", detail: domain.WorktreeDetail{
		Branch:   "feat/x",
		Failures: map[domain.DetailFamily]error{domain.DetailFamilyEnv: errors.New("git error")},
	}})

	view := model.View()
	if !strings.Contains(view, "unavailable") || !strings.Contains(view, "git error") {
		t.Error("a family that failed to read must say why: it never goes silently empty")
	}
	if !strings.Contains(view, "⚠") {
		t.Error("a failure carries the warning glyph — the legitimate-absence case never does")
	}
}

func TestBlockersRenderAboveSections(t *testing.T) {
	model := detailModel(t, domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x", IsDirty: true})
	model = update(model, detailMsg{branch: "feat/x", detail: domain.WorktreeDetail{
		Branch:   "feat/x",
		Blockers: []domain.CleanBlocker{{Label: "uncommitted changes"}},
	}})

	view := model.View()
	blockerAt := strings.Index(view, "uncommitted changes")
	linksAt := strings.Index(view, domain.DetailSectionLinks)
	if blockerAt < 0 || linksAt < 0 || blockerAt > linksAt {
		t.Error("blockers read before the sections: they answer why the action is refused")
	}
}

// TestReviewShowsTheGHFailureNotAnEmptyReview and
// TestReviewStaysAbsentWhenGHIsFineAndThereIsNoPR are the two ends of the
// distinction §8 state 4 exists to enforce: an absence caused by a broken gh
// must never render the same as an absence caused by there being nothing.
func TestReviewShowsTheGHFailureNotAnEmptyReview(t *testing.T) {
	model := detailModel(t, domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"})
	model = update(model, prsMsg{conn: domain.GHConnectionNotInstalled})

	view := model.View()
	if !strings.Contains(view, domain.DetailSectionReview) {
		t.Error("gh unavailable must still render a REVIEW section, naming why, not silence")
	}
	if !strings.Contains(view, "unavailable") {
		t.Error("the REVIEW section must say gh is unavailable, not read as an empty PR line")
	}
}

func TestReviewStaysAbsentWhenGHIsFineAndThereIsNoPR(t *testing.T) {
	model := detailModel(t, domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"})
	model = update(model, prsMsg{conn: domain.GHConnectionOK})

	if strings.Contains(model.View(), domain.DetailSectionReview) {
		t.Error("gh fine and no PR is a legitimate absence: no REVIEW section at all")
	}
}

// The keys file states the invariant: every clickable zone has a key, because
// the mouse is a convenience and the keyboard stays complete. A mouse-only
// action is unreachable over a plain ssh terminal.
func TestThePullRequestOpensFromTheKeyboardToo(t *testing.T) {
	opened := 0
	model := newTestModel(t, testWidth, testHeight, "main", "feat/a")
	model.params.PROpener = func(int) error { opened++; return nil }
	model = update(model, prsMsg{prs: []domain.PRInfo{{Branch: "main", Number: 7}}, conn: domain.GHConnectionOK})

	_, cmd := updateCmd(model, key(domain.KeyOpenPR))
	if cmd == nil {
		t.Fatal("the open-PR key must return a command")
	}
	cmd()
	if opened != 1 {
		t.Errorf("opener called %d times, want 1", opened)
	}
}

func TestTheDetailPanelShowsWhatIsRunning(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a")
	model.detailOpen = true
	model.runConfig = domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}}}
	model.jobs = []domain.JobInfo{{Name: "web", Status: domain.JobStatusRunning, WorkDir: "/tmp/a"}}
	model.details = map[string]domain.WorktreeDetail{"a": {Branch: "a"}}

	body := stripANSI(strings.Join(model.detailBody(model.layout()), "\n"))

	if !strings.Contains(body, domain.DetailSectionRun) || !strings.Contains(body, "web") {
		t.Fatalf("detail body %q must carry the RUN section", body)
	}
}

// A run config landing after the panel was built must not leave RUN missing for
// the rest of the session.
func TestANewRunConfigReloadsTheDetailOnScreen(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a")
	model.details = map[string]domain.WorktreeDetail{"a": {Branch: "a"}}

	_, cmd := updateCmd(model, jobsMsg{config: domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}}}})

	if cmd == nil {
		t.Fatal("a run config that just appeared must reload the detail built without it")
	}
}

// The same config read again is not news: reloading on every poll would mute
// the panel behind its refreshing marker while it is being read.
func TestAnUnchangedRunConfigReloadsNothing(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a")
	model.runConfig = domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}}}
	model.details = map[string]domain.WorktreeDetail{"a": {Branch: "a"}}

	_, cmd := updateCmd(model, jobsMsg{config: domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}}}})

	if cmd != nil {
		t.Fatal("the same config read again is not news: the panel must be left alone")
	}
}

func runRowsFixture() []domain.DetailRow {
	return []domain.DetailRow{
		{Key: "web", Up: true, URL: "http://web.wtm", Cells: []domain.DetailCell{
			{Kind: domain.DetailCellGlyph, Text: domain.DetailJobUpGlyph},
			{Kind: domain.DetailCellName, Text: "web"},
			{Kind: domain.DetailCellAddress, Text: "http://web.wtm"},
			{Kind: domain.DetailCellMeta, Text: "1h12m"},
		}},
		{Key: "worker", Cells: []domain.DetailCell{
			{Kind: domain.DetailCellGlyph, Text: domain.DetailJobDownGlyph},
			{Kind: domain.DetailCellName, Text: "worker"},
		}},
	}
}

func TestSectionRowLinesAlignsNamesAndFlushesMetaRight(t *testing.T) {
	lines := sectionRowLines(sectionRowLinesParams{Rows: runRowsFixture(), Width: 60})

	if len(lines) != 2 {
		t.Fatalf("lines = %d, want one per row", len(lines))
	}
	plain := stripANSI(lines[0])
	if lipgloss.Width(plain) != 60 {
		t.Errorf("width = %d, want the row padded to 60", lipgloss.Width(plain))
	}
	if !strings.HasSuffix(plain, "1h12m") {
		t.Errorf("row = %q, want the meta flush right", plain)
	}
	nameStart := strings.Index(plain, "web")
	otherStart := strings.Index(stripANSI(lines[1]), "worker")
	if nameStart != otherStart {
		t.Errorf("name column starts at %d then %d, want one column", nameStart, otherStart)
	}
	if addr := strings.Index(plain, "http://web.wtm"); addr <= nameStart+len("worker") {
		t.Errorf("address at %d, want it after the widest name", addr)
	}
}

func TestSectionRowLinesGivesTheURLTheOnlyFullIntensityStyle(t *testing.T) {
	line := sectionRowLines(sectionRowLinesParams{Rows: runRowsFixture()[:1], Width: 60})[0]

	if !strings.Contains(line, styles.DashboardURL.Render("http://web.wtm")) {
		t.Errorf("line = %q, want the url rendered through DashboardURL", line)
	}
	if !strings.Contains(line, styles.DashboardRowMeta.Render("web")) {
		t.Errorf("line = %q, want the name muted", line)
	}
}

func TestSectionRowLinesRendersANoteAsAPlainMutedLine(t *testing.T) {
	rows := []domain.DetailRow{{Cells: []domain.DetailCell{
		{Kind: domain.DetailCellNote, Text: "… 3 more"},
	}}}

	line := stripANSI(sectionRowLines(sectionRowLinesParams{Rows: rows, Width: 60})[0])

	if !strings.Contains(line, "… 3 more") {
		t.Errorf("line = %q, want the fold shown", line)
	}
	if strings.HasSuffix(line, "… 3 more") == false && strings.TrimSpace(line) != "… 3 more" {
		t.Errorf("line = %q, want the fold left-aligned, not flush right", line)
	}
}

func TestSectionRowLinesMarksATruncatedAddressAsTruncated(t *testing.T) {
	rows := []domain.DetailRow{{Key: "web", Up: true, URL: "http://a-very-long-hostname.example.test", Cells: []domain.DetailCell{
		{Kind: domain.DetailCellGlyph, Text: domain.DetailJobUpGlyph},
		{Kind: domain.DetailCellName, Text: "web"},
		{Kind: domain.DetailCellAddress, Text: "http://a-very-long-hostname.example.test"},
		{Kind: domain.DetailCellMeta, Text: "1h12m"},
	}}}

	line := stripANSI(sectionRowLines(sectionRowLinesParams{Rows: rows, Width: 40})[0])

	if lipgloss.Width(line) != 40 {
		t.Fatalf("width = %d, want 40", lipgloss.Width(line))
	}
	if !strings.Contains(line, "…") {
		t.Errorf("line = %q, want the cut marked: a clipped url reads as a whole one, and the row opens the real address", line)
	}
	if strings.Contains(line, "http://a-very-long-hostname.example.test") {
		t.Errorf("line = %q, want the address clipped to fit", line)
	}
}
