package dashboard

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

// withRunJobs is a dashboard over a project whose run.toml declares a job, which
// is what the RUN block of every menu waits for.
func withRunJobs(model Model) Model {
	return update(model, jobsMsg{config: domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}}}, known: true})
}

func labelsOf(items []menuItem) []string {
	labels := make([]string, 0, len(items))
	for _, item := range items {
		labels = append(labels, item.label)
	}
	return labels
}

// A user who never set the run module up gets back the menus v0.27.1 drew: no
// heading over a single block, no rule, no run entry.
func TestWithoutRunJobsTheMenusAreThoseOfV0271(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")
	model.statuses[0].IsParent = true

	cases := []struct {
		name string
		open func(Model) Model
		want []string
	}{
		{"base row", func(m Model) Model { return update(m, key(domain.KeyMenu)) }, []string{domain.DashboardMenuFastForward}},
		{"row", func(m Model) Model { return update(update(m, key("j")), key(domain.KeyMenu)) }, []string{
			domain.DashboardMenuFastForward, domain.DashboardMenuSync, domain.DashboardMenuReparent, domain.DashboardMenuDelete,
		}},
		{"global", func(m Model) Model { return update(m, key(domain.KeyActions)) }, []string{
			domain.DashboardMenuFastForwardAll, domain.DashboardMenuReparentBatch, domain.DashboardMenuSyncAll, domain.DashboardMenuPrune, domain.DashboardMenuDeleteMany,
		}},
	}
	for _, tc := range cases {
		got := labelsOf(tc.open(model).menuItems())
		if strings.Join(got, "|") != strings.Join(tc.want, "|") {
			t.Errorf("%s menu = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestWithRunJobsTheRunBlockIsOffered(t *testing.T) {
	model := withRunJobs(newTestModel(t, testWidth, testHeight, "a", "b"))

	row := update(model, key(domain.KeyMenu)).menuItems()
	if !hasMenuEntry(row, menuEntryAction, domain.DashboardMenuRunUp) {
		t.Errorf("row menu = %q, want the run block", labelsOf(row))
	}
	global := update(model, key(domain.KeyActions)).menuItems()
	if !hasMenuEntry(global, menuEntryAction, domain.DashboardMenuRunUpAll) {
		t.Errorf("global menu = %q, want the run block", labelsOf(global))
	}
}

// A run.toml that cannot be read is not a project without a run module: the
// block says what is wrong with the file instead of vanishing or offering
// gestures that would each refuse.
func TestAnInvalidRunTomlIsOneEntryNamingItsCause(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")
	model = update(model, jobsMsg{configErr: errors.New("line 3: expected value"), known: true})

	for name, items := range map[string][]menuItem{
		"row":    update(model, key(domain.KeyMenu)).menuItems(),
		"global": update(model, key(domain.KeyActions)).menuItems(),
	} {
		if hasMenuEntry(items, menuEntryAction, domain.DashboardMenuRunUp) || hasMenuEntry(items, menuEntryAction, domain.DashboardMenuRunUpAll) {
			t.Errorf("%s menu = %q, want no start over an unreadable run.toml", name, labelsOf(items))
		}
		entry, ok := findEntry(items, domain.DashboardRunConfigInvalid)
		if !ok {
			t.Fatalf("%s menu = %q, want %q", name, labelsOf(items), domain.DashboardRunConfigInvalid)
		}
		if entry.activatable() || !strings.Contains(entry.disabled, "line 3") {
			t.Errorf("%s entry = %+v, want an inert entry carrying the cause", name, entry)
		}
	}
}

// Stopping must work whatever run.toml says: a row with jobs up keeps its stop.
func TestAnInvalidRunTomlStillStopsWhatRuns(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")
	model = update(model, jobsMsg{
		jobs:      []domain.JobInfo{{Name: "web", Status: domain.JobStatusRunning, WorkDir: "/tmp/a"}},
		running:   map[string]int{"/tmp/a": 1},
		configErr: errors.New("bad"),
		known:     true,
	})

	items := update(model, key(domain.KeyMenu)).menuItems()
	if !hasMenuEntry(items, menuEntryAction, domain.DashboardMenuRunDown) {
		t.Errorf("menu = %q, want the stop kept for a row with jobs up", labelsOf(items))
	}
}

// The loader wraps the parser's error with the file's absolute path, which
// filled the caption before the cause could be read.
func TestTheInvalidEntryCarriesTheCauseNotThePath(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a")
	cause := errors.New("toml: line 3: expected value")
	model = update(model, jobsMsg{configErr: fmt.Errorf("parse /very/long/state/dir/run.toml: %w", cause), known: true})

	entry, _ := findEntry(update(model, key(domain.KeyMenu)).menuItems(), domain.DashboardRunConfigInvalid)
	if !strings.HasPrefix(entry.disabled, "toml: line 3") {
		t.Errorf("caption = %q, want the parser's own words", entry.disabled)
	}
}

func findEntry(items []menuItem, label string) (menuItem, bool) {
	for _, item := range items {
		if item.label == label {
			return item, true
		}
	}
	return menuItem{}, false
}

func TestLoadingJobsCarriesWhyRunTomlCannotBeRead(t *testing.T) {
	stateDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(stateDir, domain.RunFileName), []byte("[[job]\nname = "), 0o644); err != nil {
		t.Fatal(err)
	}
	model := New(t.Context(), RunParams{StateDir: stateDir, JobsLoader: func(bool) ([]domain.JobInfo, bool) { return nil, true }})
	t.Cleanup(model.Close)

	msg, ok := model.loadJobsCmd(false)().(jobsMsg)
	if !ok {
		t.Fatal("want a jobsMsg")
	}
	if msg.configErr == nil {
		t.Fatal("an unreadable run.toml must reach the model, not read as a project without jobs")
	}
}

func TestARunGestureOverAnInvalidRunTomlNamesTheCause(t *testing.T) {
	stateDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(stateDir, domain.RunFileName), []byte("[[job]\nname = "), 0o644); err != nil {
		t.Fatal(err)
	}
	model := Model{}
	model.params.StateDir = stateDir

	next, cmd := model.startRunUp(domain.WorktreeStatus{Branch: "feature", Path: "/wt/feature"})
	if cmd != nil {
		t.Fatal("nothing starts over an unreadable run.toml")
	}
	if got := strings.Join(next.outputLines, "\n"); !strings.Contains(got, domain.DashboardRunConfigInvalid) {
		t.Errorf("output = %q, want the refusal to name run.toml", got)
	}
}
