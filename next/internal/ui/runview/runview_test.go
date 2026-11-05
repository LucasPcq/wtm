package runview

import (
	"context"
	"errors"
	"io"
	"net"
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

// The daemon lets go of a client that falls behind while its job runs on: the view attaches again.
func TestAStreamThatEndedWhileItsJobRunsIsAttachedAgain(t *testing.T) {
	d := &daemon{}
	view := opened(t, d, 120, 30)
	view.keys("down", "down")
	view.send(endedMsg{conn: view.m.conn})
	view.refresh()

	assert.Equal(t, 3, d.attaches(), "db, api, then api again")
	assert.False(t, view.m.ended)
}

func TestTheOutputOfAJobThatEndedIsKept(t *testing.T) {
	d := &daemon{}
	view := opened(t, d, 120, 30)
	view.keys("down", "down")
	_, _ = view.m.pane.Write([]byte("last words\r\n"))

	_, jobs := board()
	jobs[3].Up, jobs[3].State, jobs[3].StartedAt = false, run.Stopped, time.Time{}
	d.set(jobs)
	view.send(endedMsg{conn: view.m.conn})
	view.refresh()

	assert.Equal(t, 2, d.attaches())
	assert.Contains(t, view.screen(), "last words")
}

// The emulator answers a job's queries (cursor position, device attributes) into a pipe: unread, its next write would block the view for good.
func TestAJobAskingWhereTheCursorIsDoesNotFreezeTheView(t *testing.T) {
	p := newPane(paneParams{at: size{cols: 20, rows: 4}})
	t.Cleanup(p.close)
	written := make(chan struct{})
	go func() {
		_, _ = p.Write([]byte("\x1b[6n\x1b[c\x1b]11;?\x07"))
		_, _ = p.Write([]byte("still here"))
		close(written)
	}()

	select {
	case <-written:
	case <-time.After(time.Second):
		t.Fatal("the pane's write never returned")
	}
	assert.Contains(t, formtest.Plain(p.render()), "still here")
}

// A crash notice takes rows from the pane: the pane and the job's terminal shrink with it.
func TestANoticeThatGrowsTheBandResizesThePaneAndTheJob(t *testing.T) {
	d := &daemon{}
	view := opened(t, d, 120, 30)
	view.keys("down", "down")
	_, _ = view.m.pane.Write([]byte("one\r\ntwo\r\nthree\r\n"))
	before := view.m.pane.size.rows

	_, jobs := board()
	jobs[5].Up, jobs[5].State, jobs[5].ExitCode = false, run.Crashed, &exit3
	d.set(jobs)
	view.refresh()

	require.Equal(t, before-2, view.m.pane.size.rows, "the band's notice and its dismiss line")
	assert.Equal(t, run.Window{Name: "api", WorkDir: featBDir, Cols: 87, Rows: before - 2}, d.resized[len(d.resized)-1])
	formtest.Golden(t, "band_over_output", view.screen())
}

// A write the job's terminal refuses gives the keyboard back and says why.
func TestAKeyTheJobCannotTakeEndsTheFocus(t *testing.T) {
	d := &daemon{}
	term := piped(d)
	view := opened(t, d, 120, 30)
	view.keys("down", "down", "enter")
	require.True(t, view.m.focused)
	require.NoError(t, term.job.Close())

	view.keys("x")

	assert.False(t, view.m.focused)
	assert.Contains(t, view.screen(), "! could not type into api · feat/b")
}

// The clock watches every pane, not the one it was started on: a pane selected later is drawn as it is written.
func TestAWriteToAnyPaneGivesAFrame(t *testing.T) {
	m := newModel(t.Context(), Params{})
	t.Cleanup(m.close)
	frame := m.frame()

	p := newPane(paneParams{wake: m.wake})
	t.Cleanup(p.close)
	_, _ = p.Write([]byte("ready\r\n"))
	assert.Equal(t, frameMsg{}, frame())
}

// A stream that ends at once while its job is listed up must not be attached again and again.
func TestAStreamThatEndsAtOnceIsAttachedAgainOnlyOnce(t *testing.T) {
	d := &daemon{attach: func(run.Window) (net.Conn, error) {
		view, job := net.Pipe()
		_ = job.Close()
		return view, nil
	}}
	view := opened(t, d, 120, 30)
	view.keys("down", "down")
	for range 5 {
		if conn := view.m.conn; conn != nil {
			view.send(endedMsg{conn: conn})
		}
		view.refresh()
	}

	assert.Equal(t, 4, d.attaches(), "db and api, each attached once more: %v", d.attached)
}

func TestAJobThatStartsAgainIsAttachedAgain(t *testing.T) {
	d := &daemon{}
	view := opened(t, d, 120, 30)
	view.keys("down", "down")
	_, jobs := board()
	jobs[3].StartedAt = now
	d.set(jobs)
	view.refresh()

	assert.Equal(t, 3, d.attaches(), "db, api, then api's new start")
}

func TestAStreamThatBrokeSaysWhy(t *testing.T) {
	view := opened(t, &daemon{}, 120, 30)
	view.keys("down", "down")
	view.send(endedMsg{conn: view.m.conn, err: errors.New("connection reset by peer")})

	assert.Contains(t, view.screen(), "! api · feat/b: its output stopped: connection reset by peer")
}

// A panic while a stream is read ends that stream with the panic as its error, not the process.
func TestAPanicReadingAStreamEndsItWithTheError(t *testing.T) {
	msgs := make(chan tea.Msg, 1)
	p := newPane(paneParams{})
	t.Cleanup(p.close)

	drain(drainParams{conn: panicking{}, pane: p, msgs: msgs, done: t.Context().Done()})

	ended, ok := (<-msgs).(endedMsg)
	require.True(t, ok)
	assert.ErrorContains(t, ended.err, "boom")
}

// An attach the daemon answers once the view has left is closed: nobody else would.
func TestAnAttachThatLandsAfterTheViewLeftIsClosed(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	var job net.Conn
	d := &daemon{attach: func(run.Window) (net.Conn, error) {
		cancel()
		var view net.Conn
		view, job = net.Pipe()
		return view, nil
	}}
	m := newModel(ctx, Params{Daemon: d})
	t.Cleanup(m.close)

	assert.Nil(t, m.attach(binding{key: key{dir: featBDir, name: "api"}, live: true})())
	_, err := job.Write([]byte("x"))
	assert.ErrorIs(t, err, io.ErrClosedPipe)
}

// Q5: s stops the job selected, r starts it again, each through the command the view is handed; what became of it is a notice.
func TestSStopsTheJobSelectedAndRStartsItAgain(t *testing.T) {
	stops, restarts := &acts{}, &acts{}
	view := actable(t, stops, restarts)

	view.keys("down", "down", "s")
	view.keys("r")

	assert.Equal(t, []Act{{Worktree: "feat/b", Dir: featBDir, Job: "api"}}, stops.asked)
	assert.Equal(t, []Act{{Worktree: "feat/b", Dir: featBDir, Job: "api"}}, restarts.asked)
	assert.Equal(t, []string{"api stopped", "api restarted"}, view.m.notices)
}

func TestSOnASharedServiceReleasesItAndRIsRefused(t *testing.T) {
	stops, restarts := &acts{}, &acts{}
	view := actable(t, stops, restarts)

	view.keys("s")
	view.keys("r")

	assert.Equal(t, []Act{{Worktree: "feat/b", Dir: featBDir, Job: "db"}}, stops.asked)
	assert.Empty(t, restarts.asked)
	assert.Contains(t, view.helpRow(), "db runs for every worktree holding it")
	assert.Equal(t, []string{"db released"}, view.m.notices)
}

func TestAnActionThatFailsSaysWhy(t *testing.T) {
	stops := &acts{err: errors.New("the stop command of api failed (exit 4)")}
	view := actable(t, stops, &acts{})

	view.keys("down", "down", "s")

	assert.Equal(t, []string{"could not stop api: the stop command of api failed (exit 4)"}, view.m.notices)
}

func TestAnActionUnderWayRefusesAnother(t *testing.T) {
	stops := &acts{}
	view := actable(t, stops, &acts{})
	view.keys("down", "down")

	_ = view.m.act(stopping)
	view.keys("s")

	assert.Empty(t, stops.asked)
	assert.Contains(t, view.helpRow(), "stopping api… is under way")
}

func TestAViewHandedNoStopSaysSo(t *testing.T) {
	view := opened(t, &daemon{}, 120, 30)

	view.keys("down", "down", "s")

	assert.Contains(t, view.helpRow(), "api: nothing to stop here")
}

// Q5: refresh moved from r to ctrl+r, r restarting.
func TestCtrlRListsTheJobsAgain(t *testing.T) {
	d := &daemon{}
	piped(d)
	worktrees, jobs := board()
	d.set(jobs)
	view := open(t, d, Params{Worktrees: worktrees, Main: "main"}, 120, 30)
	d.set(jobs[:1])

	view.keys("ctrl+r")

	assert.Len(t, view.m.jobs, 1)
}

func TestQWaitsForAnActionUnderWay(t *testing.T) {
	view := actable(t, &acts{}, &acts{})
	view.keys("down", "down")
	_ = view.m.act(restarting)

	view.keys("q")

	assert.False(t, view.quit)
	assert.Contains(t, view.helpRow(), "restarting api… is under way")
}

// Review: an action that had nothing to do says so, never that it stopped or released.
func TestAnActionWithNothingToDoSaysSo(t *testing.T) {
	stops := &acts{acted: Acted{Unchanged: true}}
	view := actable(t, stops, &acts{})

	view.keys("s")

	assert.Equal(t, []string{"db: nothing to release here"}, view.m.notices)
}

// Review: Ctrl+C under a restart cuts it short, waits for it to end, then leaves, saying after the view that the job is stopped and how to start it.
func TestCtrlCUnderARestartWaitsForItAndSaysTheJobIsStopped(t *testing.T) {
	restarts := &acts{cut: true}
	view := actable(t, &acts{}, restarts)
	view.keys("down", "down")
	cmd := view.m.act(restarting)

	view.keys("ctrl+c")
	assert.False(t, view.quit, "it waits for the restart")
	assert.Contains(t, view.helpRow(), "leaving once restarting api… ends")
	view.send(cmd())

	assert.True(t, view.quit)
	assert.Equal(t, "restart of api cut short: api is stopped — wtm run start feat/b --job api starts it again", view.m.left)
}

// Review: the view left before the action ended (its wait over, or Ctrl+C again) says the action still runs and how to check it: a restart by starting the job again, a stop by a read, never a start of what its user meant to stop.
func TestLeavingBeforeAnActionEndsSaysItStillRuns(t *testing.T) {
	for name, c := range map[string]struct {
		action action
		leave  func(d *driver)
		left   string
	}{
		"wait_over":    {action: restarting, leave: func(d *driver) { d.send(leaveMsg{}) }, left: "restarting api… still under way when the view was left: check it with wtm run start feat/b --job api"},
		"ctrl_c_again": {action: restarting, leave: func(d *driver) { d.keys("ctrl+c") }, left: "restarting api… still under way when the view was left: check it with wtm run start feat/b --job api"},
		"stop":         {action: stopping, leave: func(d *driver) { d.send(leaveMsg{}) }, left: "stopping api… still under way when the view was left: check it with wtm status feat/b"},
	} {
		t.Run(name, func(t *testing.T) {
			view := actable(t, &acts{}, &acts{})
			view.keys("down", "down")
			_ = view.m.act(c.action)
			view.keys("ctrl+c")

			c.leave(view)

			assert.True(t, view.quit)
			assert.Equal(t, c.left, view.m.left)
		})
	}
}

// Review: an action that fails while the view is left says why after it; one cut short before it did anything says nothing.
func TestAnActionFailingWhileTheViewIsLeftSaysWhyAfterIt(t *testing.T) {
	for name, c := range map[string]struct {
		err  error
		left string
	}{
		"failed":    {err: errors.New("start job api: spawn failed"), left: "could not restart api: start job api: spawn failed"},
		"cancelled": {err: context.Canceled},
	} {
		t.Run(name, func(t *testing.T) {
			view := actable(t, &acts{}, &acts{err: c.err})
			view.keys("down", "down")
			cmd := view.m.act(restarting)
			view.keys("ctrl+c")

			view.send(cmd())

			assert.True(t, view.quit)
			assert.Equal(t, c.left, view.m.left)
		})
	}
}
