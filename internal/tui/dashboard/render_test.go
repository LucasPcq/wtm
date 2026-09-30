package dashboard

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/LucasPcq/wtm/internal/domain"
)

// These three paths — repoName from ProjectDir, activeBranch from Cwd via
// rules.ActiveWorktree, fetchedAt from a reload — are exactly what this task
// was supposed to repair. Every other header test in this file hand-assigns
// the fields to pin the rendering; this one goes through New and a real
// worktreesMsg instead, so deleting the wiring in applyWorktrees (rather than
// just the rendering) fails it too.
func TestHeaderIsWiredThroughTheRealPath(t *testing.T) {
	const (
		projectDir = "/repo/worktree-manager-cli"
		mainPath   = "/repo/worktree-manager-cli"
		activePath = "/repo/worktree-manager-cli.worktrees/feat-a"
	)

	model := New(RunParams{ProjectDir: projectDir, Cwd: activePath})
	t.Cleanup(model.Close)
	model = update(model, tea.WindowSizeMsg{Width: testWidth, Height: testHeight})

	wtStatuses := []domain.WorktreeStatus{
		{Branch: "main", Path: mainPath},
		{Branch: "feat/a", Path: activePath},
	}
	model = update(model, worktreesMsg{
		statuses:  wtStatuses,
		parents:   map[string]string{},
		fetchedAt: time.Now().Add(-72 * time.Hour),
	})

	// The tall header spreads repo, active worktree and fetch age over three
	// rows (§ the tall header's own tests pin exactly which); this one only
	// has to prove the wiring reached the render somewhere in the block.
	header := model.renderHeader(model.layout())
	if !strings.Contains(header, "worktree-manager-cli") {
		t.Errorf("header = %q, want the repo name New() derives from ProjectDir", header)
	}
	if !strings.Contains(header, "feat/a") {
		t.Errorf("header = %q, want the worktree Cwd falls under, via rules.ActiveWorktree in applyWorktrees", header)
	}
	if !strings.Contains(header, "3 d ago") {
		t.Errorf("header = %q, want the age carried on worktreesMsg.fetchedAt", header)
	}

	// r re-runs loadWorktreesCmd, which re-reads the fetch time; applyWorktrees
	// must take the fresher value rather than keeping the first one forever, or
	// the staleness marker — once shown — would never clear.
	model = update(model, worktreesMsg{
		statuses:  wtStatuses,
		parents:   map[string]string{},
		fetchedAt: time.Now(),
	})
	if strings.Contains(model.renderHeader(model.layout()), "fetched") {
		t.Error("a fresh fetchedAt from a later worktreesMsg must clear the staleness marker")
	}
}

func TestHeaderCarriesRepoAndActiveWorktree(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main", "feat/a")
	model.repoName = "worktree-manager-cli"
	model.params.Config.Project.Worktrees.BaseBranch = "main"
	model.activeBranch = "feat/a"

	header := model.renderHeader(model.layout())
	for _, want := range []string{domain.DashboardWordmarkLines[0], "worktree-manager-cli", "main", "feat/a"} {
		if !strings.Contains(header, want) {
			t.Errorf("header = %q, doit contenir %q", header, want)
		}
	}
}

func TestHeaderAnnouncesStaleFetchOnlyPastTheThreshold(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main")

	model.fetchedAt = time.Now().Add(-time.Hour)
	if strings.Contains(model.View(), "fetched") {
		t.Error("un fetch récent ne s'annonce pas : un marqueur permanent ne signale plus rien")
	}

	model.fetchedAt = time.Now().Add(-72 * time.Hour)
	if !strings.Contains(model.View(), "fetched") {
		t.Error("au-delà du seuil, la vue doit déclarer que ses refs origin sont périmées")
	}
}

// This exercises the compact header specifically (a height below the tall
// threshold): its single packed context line is the one thing that still
// drops segments by width. The tall header spreads the same facts over
// three rows instead and does not need the same mechanic.
func TestHeaderDropsSegmentsRightToLeftWhenNarrow(t *testing.T) {
	model := newTestModel(t, narrowWide, domain.DashboardHeaderTallThreshold-1, "main")
	model.repoName = "un-nom-de-depot-vraiment-tres-long"
	model.params.Config.Project.Worktrees.BaseBranch = "main"
	model.activeBranch = "feat/une-branche-au-nom-interminable"
	model.fetchedAt = time.Now().Add(-72 * time.Hour)

	first := strings.Split(model.View(), "\n")[0]
	if lipgloss.Width(first) > narrowWide {
		t.Errorf("ligne 1 large de %d, max %d", lipgloss.Width(first), narrowWide)
	}
	if !strings.Contains(first, domain.DashboardWordmark) {
		t.Error("le wordmark est le dernier segment à tomber")
	}

	// The two checks above only prove the line fits and the wordmark survives
	// — reversing the variant order (dropping base before the active worktree,
	// say) would pass both just as well. Pin the ORDER itself: a width that
	// fits repo+base+active only without the active segment must show base
	// and drop active, never the reverse.
	order := New(RunParams{})
	t.Cleanup(order.Close)
	order.repoName, order.activeBranch = "demo-repo", "wip"
	order.params.Config.Project.Worktrees.BaseBranch = "trunk"

	const (
		widthDropsActiveOnly = 32 // " wtm  demo-repo · base trunk" (28) fits; the +active variant (36) does not.
	)
	line := order.renderContextLine(widthDropsActiveOnly)
	if !strings.Contains(line, "base trunk") {
		t.Errorf("line = %q, want base to survive at this width", line)
	}
	if strings.Contains(line, "wip") {
		t.Errorf("line = %q, want the active worktree dropped before base at this width", line)
	}
}

// TestContextLineGoesEmptyRatherThanOverflowAnUnfittableWidth pins the width
// guard directly: reverting it to "return wordmark" unconditionally would
// make this fail, since the bare wordmark itself does not fit width 4.
func TestContextLineGoesEmptyRatherThanOverflowAnUnfittableWidth(t *testing.T) {
	model := New(RunParams{})
	t.Cleanup(model.Close)

	const tooNarrowForTheWordmark = 4 // the wordmark alone renders 5 columns wide.
	if line := model.renderContextLine(tooNarrowForTheWordmark); line != "" {
		t.Errorf("renderContextLine(%d) = %q, want empty — even the bare wordmark does not fit",
			tooNarrowForTheWordmark, line)
	}
}

func TestHeaderNamesNeverFetchedDistinctlyFromAnAge(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main")

	model.fetchedAt = time.Time{}
	if !strings.Contains(model.View(), domain.DashboardNeverFetched) {
		t.Error("un dépôt jamais fetché doit le dire — le cas le plus périmé ne doit pas se taire")
	}

	model.fetchedAt = time.Now().Add(-72 * time.Hour)
	view := model.View()
	if strings.Contains(view, domain.DashboardNeverFetched) {
		t.Error("un fetch daté, même périmé, ne doit pas porter le libellé \"jamais fetché\"")
	}
	if !strings.Contains(view, "fetched") {
		t.Error("un fetch daté et périmé doit toujours annoncer son âge")
	}
}

// TestTabRuleSlidesThenSettles pins the animation's bounded shape: switching
// tabs starts it, and once its duration has elapsed the next tick both
// settles it and schedules no further work — an idle dashboard must not keep
// redrawing forever, the trap an earlier spinner fell into.
func TestTabRuleSlidesThenSettles(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main", "feat/a")

	model, cmd := model.selectTab(tabTree)
	if cmd == nil {
		t.Fatal("switching to a different tab must start the slide (and load the forest)")
	}
	if model.tabSlideSince.IsZero() {
		t.Fatal("the slide must record when it started")
	}

	model.tabSlideSince = time.Now().Add(-domain.DashboardTabSlide)
	model, cmd = updateCmd(model, tabSlideTickMsg{})
	if cmd != nil {
		t.Error("a slide whose duration has elapsed must schedule no further ticks")
	}
	if !model.tabSlideSince.IsZero() {
		t.Error("a settled slide must clear its start time")
	}
}

func TestTabRuleDoesNotSlideOnAnAlreadyActiveTab(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main", "feat/a")

	model, cmd := model.selectTab(tabWorktrees)
	if cmd != nil {
		t.Error("switching to the tab already active must start nothing")
	}
	if !model.tabSlideSince.IsZero() {
		t.Error("switching to the tab already active must not start a slide")
	}
}

func TestTabSlideDisabledWhenAnimationsOff(t *testing.T) {
	off := false
	model := New(RunParams{Config: domain.Config{Global: domain.GlobalConfig{UI: domain.UIConfig{Animations: &off}}}})
	t.Cleanup(model.Close)
	model = update(model, tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
	model = update(model, worktreesMsg{statuses: statuses("main", "feat/a"), parents: map[string]string{}})

	model, cmd := model.selectTab(tabTree)
	if !model.tabSlideSince.IsZero() {
		t.Error("ui.animations = false must start no slide")
	}
	// The forest still loads — only the animation is cut, never the action.
	if cmd == nil {
		t.Error("ui.animations = false must not swallow the tab switch itself")
	}
}

// TestRowFlashLightsThenFadesAndStops pins the same bounded shape for the
// new-worktree row flash, triggered by the existing selectBranch mechanism.
func TestRowFlashLightsThenFadesAndStops(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main")
	model.selectBranch = "feat/a"

	model, cmd := updateCmd(model, worktreesMsg{
		statuses: statuses("main", "feat/a"),
		parents:  map[string]string{},
	})
	if cmd == nil {
		t.Fatal("landing on a newly created row must start its opening flash")
	}
	if model.flashBranch != "feat/a" {
		t.Fatalf("flashBranch = %q, want feat/a", model.flashBranch)
	}

	model.flashSince = time.Now().Add(-domain.DashboardRowFlash)
	model, cmd = updateCmd(model, flashTickMsg{})
	if cmd != nil {
		t.Error("a flash whose duration has elapsed must schedule no further ticks")
	}
	if model.flashBranch != "" {
		t.Error("a settled flash must clear its branch")
	}
}

func TestRowFlashDisabledWhenAnimationsOff(t *testing.T) {
	off := false
	model := New(RunParams{Config: domain.Config{Global: domain.GlobalConfig{UI: domain.UIConfig{Animations: &off}}}})
	t.Cleanup(model.Close)
	model = update(model, tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
	model.selectBranch = "feat/a"

	// The row-selection cmd (triggerDetailReload) legitimately stays non-nil —
	// only the animation is cut, never the selection it rides along with.
	model, _ = updateCmd(model, worktreesMsg{
		statuses: statuses("main", "feat/a"),
		parents:  map[string]string{},
	})
	if model.flashBranch != "" {
		t.Error("ui.animations = false must not arm the flash at all")
	}
	if model.cursor != 1 {
		t.Error("the row must still be selected even with animations off")
	}
}

func TestSpreadNeverExceedsItsWidth(t *testing.T) {
	// A right segment that leaves no room for the left one used to fall through
	// truncateRendered(left, 0), which returns the text whole.
	line := spread("a-very-long-left-segment", "1h12m", 6)

	if got := lipgloss.Width(stripANSI(line)); got > 6 {
		t.Errorf("width = %d, want at most 6: a line wider than its panel wraps and breaks the frame", got)
	}
}

func TestSpreadKeepsTheRightSegmentWhole(t *testing.T) {
	line := stripANSI(spread("left", "1h12m", 20))

	if !strings.HasSuffix(line, "1h12m") {
		t.Errorf("line = %q, want the right segment whole and flush right", line)
	}
	if lipgloss.Width(line) != 20 {
		t.Errorf("width = %d, want 20", lipgloss.Width(line))
	}
}

// The version is permanent context; the call to action only exists when the
// cached check found something newer. Both ride the header's count row.
func TestHeaderShowsTheVersionAndCallsToActionWhenOutdated(t *testing.T) {
	cases := []struct {
		name       string
		version    string
		latest     string
		wantText   []string
		wantAbsent []string
	}{
		{
			name:     "outdated calls to action",
			version:  "0.26.0",
			latest:   "0.26.1",
			wantText: []string{"v0.26.0", "0.26.1", "run wtm upgrade"},
		},
		{
			name:       "up to date shows the version alone",
			version:    "0.26.1",
			latest:     "",
			wantText:   []string{"v0.26.1"},
			wantAbsent: []string{"run wtm upgrade"},
		},
		{
			name:       "source build never calls to action",
			version:    domain.Version,
			latest:     "0.26.1",
			wantText:   []string{"v" + domain.Version},
			wantAbsent: []string{"run wtm upgrade"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			model := New(RunParams{ProjectDir: "/repo/wtm", Version: tc.version, UpgradeLatest: tc.latest})
			t.Cleanup(model.Close)
			model = update(model, tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
			model = update(model, worktreesMsg{
				statuses: []domain.WorktreeStatus{{Branch: "main", Path: "/repo/wtm"}},
				parents:  map[string]string{},
			})

			header := stripANSI(model.renderHeader(model.layout()))
			for _, want := range tc.wantText {
				if !strings.Contains(header, want) {
					t.Errorf("header = %q, want it to mention %q", header, want)
				}
			}
			for _, absent := range tc.wantAbsent {
				if strings.Contains(header, absent) {
					t.Errorf("header = %q, must not mention %q", header, absent)
				}
			}
		})
	}
}

// The count row is hard-truncated by wordmarkRow, so it has to shed segments
// itself. The call to action is the last thing to go before the version.
func TestHeaderCountLineDropsFetchedBeforeTheCallToAction(t *testing.T) {
	model := New(RunParams{ProjectDir: "/repo/wtm", Version: "0.26.0", UpgradeLatest: "0.26.1"})
	t.Cleanup(model.Close)
	model = update(model, tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
	model = update(model, worktreesMsg{
		statuses:  []domain.WorktreeStatus{{Branch: "main", Path: "/repo/wtm"}},
		parents:   map[string]string{},
		fetchedAt: time.Now().Add(-72 * time.Hour),
	})

	roomy := stripANSI(model.headerCountLine(200))
	for _, want := range []string{"fetched", "v0.26.0", "run wtm upgrade"} {
		if !strings.Contains(roomy, want) {
			t.Fatalf("with room the line = %q, want it to carry %q", roomy, want)
		}
	}

	tight := stripANSI(model.headerCountLine(len("v0.26.0 · → 0.26.1 · run wtm upgrade") + 4))
	if strings.Contains(tight, "fetched") {
		t.Errorf("tight line = %q, want fetched dropped first — it comes back on `r`", tight)
	}
	if !strings.Contains(tight, "run wtm upgrade") {
		t.Errorf("tight line = %q, want the call to action kept", tight)
	}

	narrow := stripANSI(model.headerCountLine(len("v0.26.0")))
	if strings.Contains(narrow, "run wtm upgrade") {
		t.Errorf("narrow line = %q, want the action dropped when only the version fits", narrow)
	}
	if !strings.Contains(narrow, "v0.26.0") {
		t.Errorf("narrow line = %q, want the version to survive longest", narrow)
	}
}
