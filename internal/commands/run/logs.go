package run

import (
	"bufio"
	"fmt"
	"io"
	"sync"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/run/runctx"
	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	logsflow "github.com/LucasPcq/wtm/internal/flow/run/logs"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
)

// newLogsCmd creates the wtm run logs subcommand.
func newLogsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdLogs + " [worktree...]",
		Short: "Attach to a job's output",
		Long: "Open the run view on [worktree]'s jobs — the current worktree when omitted, picked interactively when there is a terminal.\n" +
			"--job focuses one of them; without it, every job is shown.\n" +
			"Leaving the view detaches; the jobs keep running.\n" +
			"Without a terminal, every job's output is written as prefixed lines instead.\n" +
			fmt.Sprintf("--output json replays each job's last %d lines as [{job, at, text}], grouped by job, and never attaches.", domain.JobLogTailLines),
		Args: cobra.ArbitraryArgs,
		RunE: runLogs,
	}
	shared.AddJobFlag(cmd, "Focus a single job instead of showing them all")
	shared.AddYesFlag(cmd, "Skip all prompts; shows every job of the current worktree")
	shared.AddOutputFlag(cmd)
	return cmd
}

func runLogs(cmd *cobra.Command, args []string) error {
	ctx, err := runctx.Open(runctx.OpenParams{Cmd: cmd})
	if err != nil {
		return err
	}

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	job, _ := cmd.Flags().GetString(domain.FlagJob)

	outcome, err := logsflow.Run(logsflow.Params{
		Context: ctx.FlowContext(),
		Request: logsflow.Request{
			Worktrees: args,
			Cwd:       ctx.Dir,
			Job:       job,
			Config:    ctx.Run,
		},
		Prompter:  ctx.Prompter(ctx.Interactive),
		Presenter: logsPresenter{CLIPresenter: shared.NewPresenter(cmd, format)},
	})
	if err != nil {
		return err
	}
	if outcome.Aborted {
		return domain.ErrAborted
	}
	return nil
}

// logsPresenter picks where the jobs' output goes: the full-screen view, a JSON
// document, or one prefixed line per job on a single stream.
type logsPresenter struct {
	shared.CLIPresenter
}

func (p logsPresenter) Show(show logsflow.ShowParams) error {
	params := jobLinesParams{Cmd: p.Cmd, Board: show.Board, Job: show.Job, Worktrees: show.Worktrees}
	switch rules.DecideRunSurface(rules.RunSurfaceParams{TTY: isTTY(), Format: p.Format}) {
	case domain.RunSurfaceView:
		// `run logs` starts nothing, so the view has no outcome to conclude from:
		// it only ever reports what was already running.
		_, err := showRunView(viewParams{
			Cmd: p.Cmd, Board: show.Board, Job: show.Job,
			Worktrees: show.Worktrees, Warnings: show.Warnings,
		})
		return err
	case domain.RunSurfaceMachine:
		return writeJobLogsJSON(params)
	default:
		return writeJobLines(params)
	}
}

// jobColors cycles through distinct colors for each job's log prefix.
var jobColors = []func(string) string{
	func(s string) string { return styles.Primary.Render(s) },
	func(s string) string { return styles.Success.Render(s) },
	func(s string) string { return styles.Warning.Render(s) },
	func(s string) string { return styles.Muted.Render(s) },
}

type jobLinesParams struct {
	Cmd   *cobra.Command
	Board runlogs.Board
	// Job narrows the output to one job; empty takes every job the worktree has.
	Job string
	// Worktrees are what the board covers: more than one makes each prefix name
	// where its lines came from.
	Worktrees []string
}

// prefixOf labels a job's lines, naming its worktree only above several of
// them — two jobs called `web` are otherwise the same prefix twice.
func (p jobLinesParams) prefixOf(view runlogs.JobView, index int) string {
	label := view.Name
	if len(p.Worktrees) > 1 && view.Worktree != "" {
		label = fmt.Sprintf(domain.RunStreamWorktreeFmt, label, view.Worktree)
	}
	return jobColors[index%len(jobColors)](fmt.Sprintf(domain.RunLogsPrefixFmt, label))
}

// writeJobLogsJSON is `run logs --output json`: what each job persisted, as one
// document. It never attaches, not even to a running job — an endless stream is
// not a document, and `run ps` is what reports on a job that is still going.
func writeJobLogsJSON(params jobLinesParams) error {
	if err := params.Board.Refresh(); err != nil {
		return fmt.Errorf("list jobs: %w", err)
	}

	views, err := logsflow.Views(logsflow.ViewsParams{Board: params.Board, Job: params.Job, Persisted: true})
	if err != nil {
		return err
	}

	entries := []domain.JobLogEntry{}
	for _, view := range views {
		lines, historyErr := params.Board.History(runlogs.HistoryParams{Job: view.Name, WorkDir: view.WorkDir})
		if historyErr != nil {
			// One unreadable file is not the whole document: the other jobs still
			// have something to hand over, and stdout stays a clean document
			// because the reason goes to stderr.
			output.Error(params.Cmd.ErrOrStderr(), fmt.Sprintf("%s: %v", view.Name, historyErr))
			continue
		}
		for _, line := range lines {
			entry := rules.ParseLogLine(rules.ParseLogLineParams{Job: view.Name, Line: line})
			if len(params.Worktrees) > 1 {
				entry.Worktree = view.Worktree
			}
			entries = append(entries, entry)
		}
	}
	return output.WriteJobLogsJSON(params.Cmd.OutOrStdout(), entries)
}

// writeJobLines is `run logs` with no terminal to draw on: one prefixed line per
// job on a single stream, live while the job runs and read back from its log
// file when it does not. Nothing here touches the terminal's mode — the process
// is left to die on SIGINT like any other pipe.
func writeJobLines(params jobLinesParams) error {
	if err := params.Board.Refresh(); err != nil {
		return fmt.Errorf("list jobs: %w", err)
	}

	// Persisted, like the JSON path: a job that crashed has no stream to attach
	// to, and dropping it here hid the very failure one opens the logs for.
	views, err := logsflow.Views(logsflow.ViewsParams{Board: params.Board, Job: params.Job, Persisted: true})
	if err != nil {
		return err
	}
	out := params.Cmd.OutOrStdout()
	if len(views) == 0 {
		output.Frame(out, func(w io.Writer) { output.Unchanged(w, domain.RunLogsNoJobs) })
		return nil
	}

	output.FrameStart(out)
	barred := output.Barred(out)
	writer := &lineWriter{out: barred}
	var wg sync.WaitGroup
	attached := false

	for i, view := range views {
		prefix := params.prefixOf(view, i)

		if !view.Attachable {
			lines, historyErr := params.Board.History(runlogs.HistoryParams{Job: view.Name, WorkDir: view.WorkDir})
			if historyErr != nil {
				output.Error(params.Cmd.ErrOrStderr(), fmt.Sprintf("%s: %v", view.Name, historyErr))
				continue
			}
			for _, line := range lines {
				writer.write(prefix, line)
			}
			continue
		}

		stream, attachErr := params.Board.Attach(runlogs.AttachParams{Job: view.Name, WorkDir: view.WorkDir})
		if attachErr != nil {
			output.Error(params.Cmd.ErrOrStderr(), fmt.Sprintf("%s: %v", view.Name, attachErr))
			continue
		}

		attached = true
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer stream.Close()
			scanner := bufio.NewScanner(&streamReader{chunks: stream.Chunks()})
			for scanner.Scan() {
				writer.write(prefix, scanner.Text())
			}
		}()
	}

	wg.Wait()
	// A worktree whose jobs are all down and none of which ever wrote a line has
	// nothing to show; saying so beats an empty frame.
	if !attached && !writer.wrote {
		output.Unchanged(barred, domain.RunLogsNoJobs)
	}
	output.FrameEnd(out)
	return nil
}

// lineWriter serializes the lines several jobs write at once: without it two
// prefixes and two lines interleave inside one row.
type lineWriter struct {
	mu    sync.Mutex
	out   io.Writer
	wrote bool
}

func (w *lineWriter) write(prefix string, line string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.wrote = true
	fmt.Fprintf(w.out, "%s%s %s\n", output.Indent, prefix, line)
}

// streamReader reads a job's chunks as the io.Reader a line scanner needs.
type streamReader struct {
	chunks  <-chan []byte
	pending []byte
}

// Read never writes into a chunk: it is shared with the job's other
// subscribers, so what does not fit in p is kept as a slice of it until the
// next call.
func (r *streamReader) Read(p []byte) (int, error) {
	for len(r.pending) == 0 {
		chunk, open := <-r.chunks
		if !open {
			return 0, io.EOF
		}
		r.pending = chunk
	}
	n := copy(p, r.pending)
	r.pending = r.pending[n:]
	return n, nil
}
