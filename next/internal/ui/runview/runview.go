// Package runview is the live view of the jobs of some worktrees: their list,
// the shared services first, then each worktree's own jobs, and the terminal of
// the job selected, attached through the run daemon. Leaving it stops nothing.
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

// Daemon is the run daemon as the view asks it: the jobs it runs, a job's
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
// notices its band opens on, how a URL is opened (nil opens none), and the
// terminal it takes over.
type Params struct {
	Daemon    Daemon
	Worktrees []Worktree
	Main      string
	Job       string
	Notices   []string
	Open      func(url string) error
	In        io.Reader
	Out       io.Writer
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
	return nil
}
