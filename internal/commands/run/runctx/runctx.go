// Package runctx is what every command of the `run` module opens on: where it
// was launched, the repository's config, its run.toml, the module's opt-in
// guard, and the prompt-capability gate. It exists once because the five were
// written out at every entry point in three spellings that could drift apart —
// and one of them, the guard, is a refusal.
package runctx

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/service/runconfig"
)

// IsTTY reports whether the command owns a terminal. It is a variable so a test
// can answer yes without one.
var IsTTY = func() bool { return term.IsTerminal(int(os.Stdin.Fd())) }

type Context struct {
	// Dir is the directory the command was launched from, resolved once.
	Dir    string
	Config shared.ConfigResult
	Run    domain.RunConfig
	Format string
	// Interactive is the prompt-capability gate in the one spelling the whole
	// CLI uses: a human format, on a terminal, and not bypassed by --yes.
	Interactive bool
}

type OpenParams struct {
	Cmd *cobra.Command
	// Dir is where to read the configuration from, defaulting to the current
	// directory. `run list`'s picker names it explicitly: the action it
	// dispatches acts on the worktree the listing was about, not on wherever it
	// ran.
	Dir string
	// SkipGuard leaves the opt-in guard out, for the two `add` commands: a
	// run.toml declaring nothing yet is exactly what they are for.
	SkipGuard bool
}

func Open(params OpenParams) (Context, error) {
	dir := params.Dir
	if dir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return Context{}, fmt.Errorf("get working directory: %w", err)
		}
		dir = wd
	}

	result, err := shared.LoadConfig(params.Cmd, dir)
	if err != nil {
		return Context{}, err
	}
	cfg, err := runconfig.Load(result.StateDir)
	if err != nil {
		return Context{}, fmt.Errorf("load run.toml: %w", err)
	}
	if !params.SkipGuard {
		if err := shared.RequireRunInitialized(cfg); err != nil {
			return Context{}, err
		}
	}

	format, _ := params.Cmd.Flags().GetString(domain.FlagOutput)
	yes, _ := params.Cmd.Flags().GetBool(domain.FlagYes)
	return Context{
		Dir:    dir,
		Config: result,
		Run:    cfg,
		Format: format,
		Interactive: shared.Interactive(shared.UnattendedParams{
			TTY: IsTTY(), Format: format, Yes: yes,
		}),
	}, nil
}

func (c Context) FlowContext() flow.Context { return shared.FlowContext(c.Config) }

// Prompter installs the questioning seam the way every runner of the module
// does: on stderr, so a command whose stdout is consumed still has somewhere to
// ask. The gate is passed rather than read from the context because `run down
// --all` narrows it further — it takes neither a worktree nor a profile, so
// there is nothing left to ask about.
func (c Context) Prompter(interactive bool) flow.Prompter {
	return shared.FlowPrompter(shared.FlowPrompterParams{Interactive: interactive, Stderr: true})
}

func (c Context) CLI(cmd *cobra.Command) shared.CLIPresenter {
	return shared.NewPresenter(cmd, c.Format)
}

type ListingParams struct {
	Cmd *cobra.Command
	// JSON is the document form, Table the one a reader gets. A listing nobody
	// can act on is a listing: without a terminal the picker has no second half,
	// so the table is the whole answer.
	JSON  func(io.Writer) error
	Table func(io.Writer)
}

// Listing writes the non-interactive halves of a `list` command and reports
// whether it answered. False means the picker is what the caller wanted.
func (c Context) Listing(params ListingParams) (bool, error) {
	out := params.Cmd.OutOrStdout()
	if c.Format == domain.OutputJSON {
		return true, params.JSON(out)
	}
	if c.Interactive {
		return false, nil
	}
	output.Frame(out, func(w io.Writer) { params.Table(w) })
	return true, nil
}

// FirstArg is the positional subject these commands take, empty when it was not
// given and the picker has to answer for it.
func FirstArg(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}
