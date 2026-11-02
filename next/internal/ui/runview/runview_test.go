package runview

import (
	"strings"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheFrameAtEachSizeHasItsGolden(t *testing.T) {
	for name, at := range map[string]struct{ width, height int }{
		"wide":     {width: 120, height: 30},
		"narrow":   {width: 80, height: 20},
		"listless": {width: 60, height: 16},
	} {
		t.Run(name, func(t *testing.T) {
			d := opened(t, &daemon{}, at.width, at.height)

			formtest.Golden(t, "frame_"+name, d.screen())
		})
	}
}

// B10: the legacy view listed a shared service only once joined, and titled a
// running task stopped.
func TestTheBoardListsEveryDeclaredJobTheSharedServicesFirst(t *testing.T) {
	d := opened(t, &daemon{}, 120, 30)
	d.keys("down", "down", "down", "down")

	formtest.Golden(t, "board_running_task", d.screen())
	assert.Contains(t, d.screen(), "seed · feat/a · running")
}

func TestAJobItsWorktreeDeclaresButTheDaemonDoesNotKnowIsNotStarted(t *testing.T) {
	d := opened(t, &daemon{}, 120, 30)
	d.keys("down", "down", "down")

	assert.Contains(t, d.screen(), "web · feat/b · not started")
	assert.Contains(t, d.screen(), "Not started.")
}

func TestAJobOfAnotherRepositoryIsNotListed(t *testing.T) {
	d := &daemon{}
	piped(d)
	view := opened(t, d, 120, 30)

	assert.Contains(t, view.helpRow(), "2 worktrees · 4/6 running")
	assert.NotContains(t, view.screen(), "elsewhere")
}

// B9: the legacy focus hint printed the view's internal key, the path and the job run together.
func TestTheFocusHintNamesTheJobAndItsWorktree(t *testing.T) {
	d := &daemon{}
	piped(d)
	view := opened(t, d, 120, 30)
	view.keys("down", "down", "enter")

	require.True(t, view.m.focused, view.screen())
	formtest.Golden(t, "focus_hint", view.helpRow())
	assert.NotContains(t, view.screen(), featBDir)
}

// B14: focus on a running task was accepted and its keys dropped.
func TestFocusIsRefusedOnATaskAndOnASharedServiceAnotherWorktreeRuns(t *testing.T) {
	for name, c := range map[string]struct {
		keys []string
		said string
	}{
		"task":  {keys: []string{"down", "down", "down", "down", "enter"}, said: "seed runs under no terminal: nothing to type into."},
		"claim": {keys: []string{"enter"}, said: "db runs in main: its output is read-only here."},
	} {
		t.Run(name, func(t *testing.T) {
			d := &daemon{}
			piped(d)
			view := opened(t, d, 120, 30)
			view.keys(c.keys...)

			assert.False(t, view.m.focused)
			assert.Contains(t, view.helpRow(), c.said)
		})
	}
}

// Q2: a claim focused is refused, its lines visible.
func TestTheSharedServiceAWorktreeHoldsShowsItsOutput(t *testing.T) {
	d := &daemon{}
	term := piped(d)
	view := opened(t, d, 120, 30)

	_, err := term.job.Write([]byte("database system is ready\r\n"))
	require.NoError(t, err)
	view.waitFor("database system is ready")
	assert.Equal(t, run.Window{Name: "db", WorkDir: featBDir, Cols: 87, Rows: 23}, d.attached[0])
}

func TestFocusTypesEveryKeyIntoTheJobAndEscTwiceGivesItBack(t *testing.T) {
	d := &daemon{}
	term := piped(d)
	view := opened(t, d, 120, 30)
	view.keys("down", "down", "enter", "x", "ctrl+c")
	view.m.now = func() time.Time { return now.Add(time.Second) }
	view.keys("esc")

	assert.Equal(t, "x", term.read(t))
	assert.Equal(t, "\x03", term.read(t))
	assert.Equal(t, "\x1b", term.read(t))
	require.True(t, view.m.focused, "a lone esc is the job's")

	view.m.now = func() time.Time { return now.Add(time.Second + 300*time.Millisecond) }
	view.keys("esc")
	assert.False(t, view.m.focused)
	assert.False(t, view.quit, "ctrl+c went to the job")
}

// B14: the legacy view attached again on every listing, every 2 s, once an attach failed.
func TestAFailedAttachIsNotTriedAgainUntilTheJobStartsAgain(t *testing.T) {
	d := &daemon{}
	worktrees, jobs := board()
	d.set(jobs)
	view := open(t, d, Params{Worktrees: worktrees, Main: "main"}, 120, 30)
	view.keys("down", "down")
	view.refresh()
	view.refresh()

	assert.Equal(t, 2, d.attaches(), "db once, api once")
	assert.Contains(t, view.helpRow(), errNoTerminal.Error())
	assert.Contains(t, view.screen(), "No output to show: attach refused")

	jobs[3].StartedAt = now
	d.set(jobs)
	view.refresh()
	assert.Equal(t, 3, d.attaches())
}

// B11: dismissing the band in the legacy view hid every later report.
func TestABandDismissedComesBackWithACrashSinceTheViewOpened(t *testing.T) {
	d := &daemon{}
	piped(d)
	worktrees, jobs := board()
	d.set(jobs)
	view := open(t, d, Params{Worktrees: worktrees, Main: "main", Notices: []string{"web's port 3000 never answered"}}, 120, 30)
	require.Contains(t, view.screen(), "! web's port 3000 never answered")

	view.keys("esc")
	require.NotContains(t, view.screen(), "never answered")

	crashed := jobs[3]
	crashed.Up, crashed.State, crashed.ExitCode = false, run.Crashed, &exit3
	jobs[3] = crashed
	d.set(jobs)
	view.refresh()

	assert.Contains(t, view.screen(), "! api · feat/b crashed (exit 3) since the view opened")
	assert.Contains(t, view.screen(), "web's port 3000 never answered")
	formtest.Golden(t, "band", view.screen())
}

func TestAJobThatEndedShowsItsLog(t *testing.T) {
	d := &daemon{logs: map[string][]string{"migrate": {"applying 0001_users", "done"}}}
	view := opened(t, d, 120, 30)
	view.keys("down")

	assert.Contains(t, view.screen(), "migrate · feat/b · done")
	assert.Contains(t, view.screen(), "history")
	assert.Contains(t, view.screen(), "applying 0001_users")
	assert.Equal(t, 1, d.attaches(), "only db was attached")
}

func TestTheFilterNarrowsTheListAndMovesTheCursorOffAJobItHides(t *testing.T) {
	view := opened(t, &daemon{}, 120, 30)
	view.keys("/", "a", "p")

	assert.Contains(t, view.helpRow(), "filter: ap")
	assert.NotContains(t, view.screen(), "migrate")
	assert.Equal(t, key{dir: featBDir, name: "api"}, view.m.selected)

	view.keys("esc")
	assert.Contains(t, view.screen(), "migrate")
}

// The legacy view's help said `esc clears` under a filter applied, and esc left it.
func TestEscClearsAFilterApplied(t *testing.T) {
	view := opened(t, &daemon{}, 120, 30)
	view.keys("/", "a", "p", "enter")
	require.Contains(t, view.helpRow(), `filter "ap" · esc clears`)

	view.keys("esc")
	assert.NotContains(t, view.helpRow(), `filter "ap"`)
	assert.Contains(t, view.screen(), "migrate")
}

func TestOOpensTheURLOfTheJobSelectedAndAClickTheURLUnderIt(t *testing.T) {
	var opened []string
	d := &daemon{}
	worktrees, jobs := board()
	d.set(jobs)
	view := open(t, d, Params{Worktrees: worktrees, Main: "main", Open: func(url string) error { opened = append(opened, url); return nil }}, 120, 30)
	view.keys("down", "down", "o")
	require.Equal(t, []string{"http://api.feat-b.acme.localhost:11080"}, opened)

	view.keys("a")
	y, x := locate(t, view.m.View(), "http://api.feat-a")
	view.send(tea.MouseMsg{X: x, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionRelease})
	assert.Equal(t, "http://api.feat-a.acme.localhost:11080", opened[1])
}

func TestAddressesListWhereEachJobUpAnswers(t *testing.T) {
	view := opened(t, &daemon{}, 120, 30)
	view.keys("a")

	formtest.Golden(t, "addresses", view.screen())
}

func TestTheWheelMovesTheCursorOverTheListAndScrollsThePaneElsewhere(t *testing.T) {
	view := opened(t, &daemon{}, 120, 30)
	view.send(tea.MouseMsg{X: 5, Y: 8, Button: tea.MouseButtonWheelDown})

	assert.Equal(t, key{dir: featBDir, name: "migrate"}, view.m.selected)
}

func TestQAndCtrlCLeaveTheView(t *testing.T) {
	for _, k := range []string{"q", "ctrl+c"} {
		t.Run(strings.ReplaceAll(k, "+", "_"), func(t *testing.T) {
			view := opened(t, &daemon{}, 120, 30)
			view.keys(k)

			assert.True(t, view.quit)
		})
	}
}

func TestAKeyIsTypedAsATerminalSendsIt(t *testing.T) {
	for name, c := range map[string]struct {
		key  tea.KeyMsg
		want string
	}{
		"runes":    {key: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("é")}, want: "é"},
		"alt":      {key: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b"), Alt: true}, want: "\x1bb"},
		"enter":    {key: tea.KeyMsg{Type: tea.KeyEnter}, want: "\r"},
		"ctrl_a":   {key: tea.KeyMsg{Type: tea.KeyCtrlA}, want: "\x01"},
		"up":       {key: tea.KeyMsg{Type: tea.KeyUp}, want: "\x1b[A"},
		"ctrl_end": {key: tea.KeyMsg{Type: tea.KeyCtrlEnd}, want: "\x1b[1;5F"},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, c.want, string(encode(c.key)))
		})
	}
}

func TestAPipesLineFeedsAreGivenTheCarriageReturnATerminalAdds(t *testing.T) {
	p := newPane(paneParams{at: size{cols: 20, rows: 4}, crlf: true})
	_, _ = p.Write([]byte("one\ntwo\r"))
	_, _ = p.Write([]byte("\nthree\n"))

	assert.Equal(t, "one\ntwo\nthree", formtest.Plain(p.render()))
}

func TestAPaneKeepsTheReaderOnTheLinesTheyScrolledBackTo(t *testing.T) {
	p := newPane(paneParams{at: size{cols: 20, rows: 2}, crlf: true})
	_, _ = p.Write([]byte("1\n2\n3\n4\n"))
	p.scroll(2)
	_, _ = p.Write([]byte("5\n"))

	assert.Equal(t, 3, p.scrolled())
	assert.Equal(t, "2\n3", formtest.Plain(p.render()))
}

func TestTheLayoutDropsTheListBeforeThePaneGrowsUnreadable(t *testing.T) {
	assert.Equal(t, 26, layoutAt(120, 30, 0).sidebar.width)
	assert.Zero(t, layoutAt(60, 30, 0).sidebar.width)
	assert.Equal(t, 3, layoutAt(120, 30, 3).notice.height)
	assert.Equal(t, 2, layoutAt(120, 6, 3).notice.height, "the band takes no row the body needs")
}

func TestAJobThatRunsAgainIsAttachedAgainOnceItsStreamEnded(t *testing.T) {
	d := &daemon{}
	view := opened(t, d, 120, 30)
	view.keys("down", "down")
	view.send(endedMsg{conn: view.m.conn})
	require.True(t, view.m.ended)
	view.refresh()
	require.Equal(t, 2, d.attaches(), "an ended stream is kept while the job is the same")

	_, jobs := board()
	jobs[3].StartedAt = now
	d.set(jobs)
	view.refresh()

	assert.Equal(t, 3, d.attaches(), "db, api, then api again")
	assert.False(t, view.m.ended)
}

// The clock watches every pane, not the one it was started on: a pane selected later is drawn as it is written.
func TestAWriteToAnyPaneGivesAFrame(t *testing.T) {
	m := New(t.Context(), Params{})
	frame := m.frame()

	_, _ = newPane(paneParams{wake: m.wake}).Write([]byte("ready\r\n"))
	assert.Equal(t, frameMsg{}, frame())
}
