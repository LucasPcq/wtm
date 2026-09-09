package resolve

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/tui/components"
	"github.com/LucasPcq/wtm/internal/tui/worktreepicker"
)

// NewCmd creates the wtm resolve command. Its bare-path stdout is what the shell
// wrapper (`wtm go`) consumes; `--output json` emits {path, branch} for scripts.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resolve [branch]",
		Short: "Resolve a branch to its worktree path",
		RunE:  runResolve,
	}
	shared.AddOutputFlag(cmd)
	return cmd
}

func runResolve(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	root, err := shared.ProjectRoot(dir)
	if err != nil {
		return err
	}

	query := strings.Join(args, " ")
	format, _ := cmd.Flags().GetString(domain.FlagOutput)

	result, err := worktree.Resolve(domain.ResolveParams{
		ProjectDir: root,
		Query:      query,
	})
	if errors.Is(err, domain.ErrWorktreeNotFound) {
		if format == domain.OutputJSON {
			return err
		}
		output.Frame(cmd.ErrOrStderr(), func(w io.Writer) {
			output.Warning(w, fmt.Sprintf("No worktree found matching %q", query))
		})
		return nil
	}
	if err != nil {
		return err
	}

	if !result.Ambiguous {
		return emitResolved(cmd, format, result.Path, result.Branch)
	}

	// A picker needs a TTY, which JSON mode cannot assume; ask the caller to
	// narrow the query instead of silently guessing.
	if format == domain.OutputJSON {
		return fmt.Errorf("%q is ambiguous: narrow the query to a single worktree", query)
	}

	selected, pickErr := pickAmbiguousWorktree(cmd, dir, root, result.Matches)
	if errors.Is(pickErr, domain.ErrUserAborted) {
		return nil
	}
	if pickErr != nil {
		return pickErr
	}

	return emitResolved(cmd, format, selected.Path, selected.Branch)
}

// emitResolved writes the resolved worktree as bare-path text (what the shell
// wrapper reads) or as {path, branch} JSON.
func emitResolved(cmd *cobra.Command, format string, path string, branch string) error {
	if format == domain.OutputJSON {
		return output.WriteResolveJSON(cmd.OutOrStdout(), output.ResolveJSON{Path: path, Branch: branch})
	}
	fmt.Fprintln(cmd.OutOrStdout(), path)
	return nil
}

func pickAmbiguousWorktree(cmd *cobra.Command, cwd, projectDir string, matches []domain.GitWorktree) (domain.WorktreeStatus, error) {
	cfgResult, err := shared.LoadConfig(cmd, projectDir)
	if err != nil {
		return domain.WorktreeStatus{}, err
	}

	// Load worktree statuses and running services (both fast/local) in parallel
	// behind a single spinner. PRs (the slow gh call) are fetched lazily inside
	// the picker so the list shows instantly. Everything goes to stderr: stdout
	// is reserved for the resolved path the shell wrapper reads.
	var (
		statuses []domain.WorktreeStatus
		listErr  error
		services []domain.JobInfo
		wg       sync.WaitGroup
	)

	loadErr := components.RunLoading(components.LoadingParams{
		Message: "Loading worktrees…",
		Animate: true,
		Work: func() error {
			wg.Add(2)
			go func() {
				defer wg.Done()
				statuses, listErr = worktree.List(domain.ListParams{
					ProjectDir: cfgResult.ProjectDir,
					StateDir:   cfgResult.StateDir,
					Config:     cfgResult.Config,
				})
			}()
			go func() {
				defer wg.Done()
				services = shared.LoadJobsGraceful()
			}()
			wg.Wait()
			return listErr
		},
	})
	if loadErr != nil {
		return domain.WorktreeStatus{}, fmt.Errorf("list worktrees: %w", loadErr)
	}

	filtered := rules.FilterStatusesByMatches(statuses, matches)
	if len(filtered) == 0 {
		filtered = statuses
	}

	return worktreepicker.Run(worktreepicker.RunParams{
		Statuses:     filtered,
		Services:     services,
		Title:        "Select a worktree",
		ProjectDir:   cfgResult.ProjectDir,
		StateDir:     cfgResult.StateDir,
		Config:       cfgResult.Config,
		ActiveBranch: rules.ActiveWorktree(rules.ActiveWorktreeParams{Cwd: infra.ResolvePath(cwd), Statuses: statuses}),
		PRLoader: func() ([]domain.PRInfo, domain.GHConnection) {
			return shared.LoadPRs(cfgResult.ProjectDir)
		},
	})
}
