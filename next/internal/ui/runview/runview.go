// Package runview is the live view of the jobs of some worktrees: their list,
// the shared services first, then each worktree's own jobs, and the terminal of
// the job selected, attached through wtm's daemon. Leaving it stops nothing.
package runview

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
	tea "github.com/charmbracelet/bubbletea"
)

// Daemon is wtm's daemon as the view asks it: the jobs it runs, a job's
// terminal attached and sized, and the lines a job left in its log.
type Daemon interface {
	Jobs(ctx context.Context) ([]run.Job, error)
	Attach(ctx context.Context, req run.Window) (net.Conn, error)
	Resize(ctx context.Context, req run.Window) error
	Lines(ctx context.Context, req run.Tail, out func(run.Line)) (run.Job, error)
}

// A Worktree is one the view shows the jobs of: its branch, which heads its
// group (its path when it has none), its path, the jobs it declares, in
// order, listed even before the daemon knows them, and the namespace it holds
// in each shared service, by the service's name.
type Worktree struct {
	Branch     string
	Path       string
	Jobs       []string
	Namespaces map[string]string
}

// Params is what the view shows and where: the daemon it asks, the worktrees
// whose jobs it lists, the branch of the main checkout the shared services run
// in, the job selected first (by name; empty selects the first listed), the
// notices its band opens on, how a URL is opened (nil opens none), how a job
// is stopped and started again (nil: the key does nothing), the command lines
// that start it again and show how it stands, and the terminal it takes over.
type Params struct {
	Daemon    Daemon
	Worktrees []Worktree
	Main      string
	Job       string
	Notices   []string
	Open      func(url string) error
	Stop      Acting
	Restart   Acting
	Again     func(act Act) string
	Show      func(act Act) string
	In        io.Reader
	Out       io.Writer
}

// A View is where a command shows the jobs it started: wtm's daemon the
// view asks, how it opens an address in the browser (nil opens none), how it
// stops a job and starts it again (nil: s and r do nothing), and the command
// lines the view gives when it is left with an action cut short or still under
// way: the one that starts a job again (after a restart), and the read that
// shows how a job stands (after a stop). A zero View shows none.
type View struct {
	Daemon  Daemon
	Open    func(url string) error
	Stop    Acting
	Restart Acting
	Again   func(act Act) string
	Show    func(act Act) string
}

// Params are the params of a view over v: its daemon, and how it opens,
// stops and starts again.
func (v View) Params() Params {
	return Params{Daemon: v.Daemon, Open: v.Open, Stop: v.Stop, Restart: v.Restart, Again: v.Again, Show: v.Show}
}

// An Act is a job a key acts on: the worktree it is listed under, by its
// branch (its path on no branch), that worktree's root, and the job's name.
type Act struct {
	Worktree string
	Dir      string
	Job      string
}

// Acting stops a job, or starts it again, as the command line does, and says
// what it did; it fails with why it did not, context.Canceled when it was cut
// short before it did anything.
type Acting func(ctx context.Context, act Act) (Acted, error)

// Acted is what an action did: nothing when there was nothing to do (a job
// not up, a shared service its worktree does not hold), or, cut short, a
// restart that stopped the job and did not start it again.
type Acted struct {
	Unchanged bool
	CutShort  bool
}

// Run shows the view on the alternate screen until the user leaves it, with q
// or Ctrl+C; every job keeps running. It fails only when the terminal cannot
// be taken over.
func Run(ctx context.Context, params Params) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	m := newModel(ctx, params)
	defer m.close()

	// The terminal's own file, not the stream around it: Bubble Tea sizes the view, and follows its resizes, only on a file.
	out := params.Out
	if stream, ok := out.(*output.Stream); ok {
		out = stream.Writer
	}
	program := tea.NewProgram(m, tea.WithContext(ctx), tea.WithAltScreen(), tea.WithMouseCellMotion(), tea.WithInput(params.In), tea.WithOutput(out))
	if _, err := program.Run(); err != nil && !errors.Is(err, tea.ErrInterrupted) && !errors.Is(err, tea.ErrProgramKilled) {
		return fmt.Errorf("live view: %w", err)
	}
	// Said once the screen is given back: what the user must know after leaving.
	if m.left != "" {
		_, _ = fmt.Fprintln(params.Out, m.left)
	}
	return nil
}
