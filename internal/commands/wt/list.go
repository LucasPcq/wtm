package wt

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	ghservice "github.com/LucasPcq/wtm/internal/service/github"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/tui/components"
	"github.com/LucasPcq/wtm/internal/tui/worktreepicker"
	"github.com/LucasPcq/wtm/internal/tui/worktreerefresh"
)

// newListCmd creates the wtm list subcommand.
func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdList,
		Short: "List all worktrees",
		Long:  "List all git worktrees with their status, PR info, and running services.",
		RunE:  runList,
	}
	shared.AddOutputFlag(cmd)
	cmd.Flags().Bool(domain.FlagWithPRs, false, "Include GitHub PR info in non-interactive output (fetched eagerly)")
	return cmd
}

func runList(cmd *cobra.Command, _ []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	result, err := shared.LoadConfig(cmd, dir)
	if err != nil {
		return err
	}

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	withPRs, _ := cmd.Flags().GetBool(domain.FlagWithPRs)
	interactive := rules.IsHumanFormat(format) && term.IsTerminal(int(os.Stdin.Fd()))

	// Load worktree statuses and running services (both fast/local) in parallel.
	// PRs (the slow gh call) are deferred: streamed into the picker interactively,
	// or fetched only on demand (--with-prs) in non-interactive mode.
	var (
		statuses []domain.WorktreeStatus
		listErr  error
		services []domain.JobInfo
		wg       sync.WaitGroup
	)

	err = components.RunLoading(components.LoadingParams{
		Message: "Loading worktrees…",
		Animate: shared.Animate(cmd, rules.IsHumanFormat(format)),
		Work: func() error {
			wg.Add(2)
			go func() {
				defer wg.Done()
				statuses, listErr = worktree.List(domain.ListParams{
					ProjectDir: result.ProjectDir,
					StateDir:   result.StateDir,
					Config:     result.Config,
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
	if err != nil {
		return fmt.Errorf("list worktrees: %w", err)
	}

	activeBranch := rules.ActiveWorktree(rules.ActiveWorktreeParams{Cwd: infra.ResolvePath(dir), Statuses: statuses})

	// Non-interactive (JSON or piped text) behaves identically across formats:
	// PRs are included only with --with-prs, since a pipe can't stream a loader.
	if !interactive {
		var prs []domain.PRInfo
		if withPRs {
			prs, _ = shared.LoadPRs(result.ProjectDir)
		}
		if format == domain.OutputJSON {
			return output.WriteWorktreeListJSON(cmd.OutOrStdout(), output.WriteWorktreeListJSONParams{
				Statuses: statuses,
				PRInfos:  prs,
				Services: services,
			})
		}
		output.Frame(cmd.OutOrStdout(), func(w io.Writer) {
			fmt.Fprintln(w, strings.TrimRight(output.FormatWorktreeList(output.FormatWorktreeListParams{
				Statuses:     statuses,
				ActiveBranch: activeBranch,
				PRInfos:      prs,
				Services:     services,
			}), "\n"))
		})
		return nil
	}

	if len(statuses) == 0 {
		output.Frame(cmd.OutOrStdout(), func(w io.Writer) {
			output.Unchanged(w, domain.NoWorktreesMessage)
		})
		return nil
	}

	selected, action, prs, err := pickWorktreeAndAction(pickParams{
		statuses:     statuses,
		services:     services,
		projectDir:   result.ProjectDir,
		stateDir:     result.StateDir,
		config:       result.Config,
		activeBranch: activeBranch,
	})
	if errors.Is(err, domain.ErrUserAborted) {
		return nil
	}
	if err != nil {
		return err
	}

	return executeWorktreeAction(cmd, action, selected, prs, result)
}

const (
	lsActionGo           = "go"
	lsActionOpenPR       = "open-pr"
	lsActionServicesUp   = "services-up"
	lsActionServicesDown = "services-down"
	lsActionLogs         = "logs"
	lsActionClean        = "clean"
)

// findPRForBranch returns the open PR whose head matches the given branch.
func findPRForBranch(prs []domain.PRInfo, branch string) (domain.PRInfo, bool) {
	for _, pr := range prs {
		if pr.Branch == branch {
			return pr, true
		}
	}
	return domain.PRInfo{}, false
}

// pickParams holds the inputs for the worktree/action wizard.
type pickParams struct {
	statuses     []domain.WorktreeStatus
	services     []domain.JobInfo
	projectDir   string
	stateDir     string
	config       domain.Config
	activeBranch string
}

// pickWorktreeAndAction renders the worktree picker instantly and streams PRs
// in asynchronously, refreshing the row badges when they arrive. It returns the
// selected worktree, the chosen action, and the PRs loaded by the time the user
// confirmed (used to resolve the Open PR action).
func pickWorktreeAndAction(params pickParams) (domain.WorktreeStatus, string, []domain.PRInfo, error) {
	statuses := params.statuses

	var (
		loadedPRs  []domain.PRInfo
		loadedConn domain.GHConnection
		prsLoaded  bool
	)

	// holder backs the worktree rows and is swapped in place on "r" refresh
	// (re-fetch origin + re-list); buildWtItems reads through it and loadedPRs so a
	// refresh keeps the streamed PR badges. Closures below read the `statuses`
	// variable (not a copy) so a refresh reaching them sees the fresh slice.
	holder := &statuses
	buildWtItems := func() []components.SelectItem {
		items := make([]components.SelectItem, 0, len(*holder))
		for i, s := range *holder {
			status := worktreepicker.BuildStatus(s)
			items = append(items, components.SelectItem{
				Label: s.Branch,
				Value: strconv.Itoa(i),
				Badges: worktreepicker.BuildTags(worktreepicker.BuildTagsParams{
					Status:       s,
					PRs:          loadedPRs,
					Services:     params.services,
					ActiveBranch: params.activeBranch,
				}),
				Status: &status,
			})
		}
		return items
	}

	branchForValue := func(v string) string {
		idx, err := strconv.Atoi(v)
		if err != nil || idx < 0 || idx >= len(*holder) {
			return v
		}
		return (*holder)[idx].Branch
	}

	listParams := domain.ListParams{ProjectDir: params.projectDir, StateDir: params.stateDir, Config: params.config}

	wiz := components.NewWizardWithParams(components.WizardParams{
		Steps: []components.Step{
			{
				Name:  "Worktree",
				Model: components.NewSelectList(components.NewSelectListParams{Title: "Select a worktree", Items: buildWtItems()}),
				Build: func([]components.Step) any {
					return components.NewSelectList(components.NewSelectListParams{Title: "Select a worktree", Items: buildWtItems()})
				},
				CanRefresh: true,
				Summary: func(m any) string {
					sl, ok := m.(components.SelectListModel)
					if !ok {
						return ""
					}
					return branchForValue(sl.Value())
				},
			},
			{
				Name:  "Action",
				Model: components.NewSelectList(components.NewSelectListParams{Title: "Action"}),
				Build: func(prev []components.Step) any {
					selected := selectedWorktree(prev, statuses)
					items := buildActionItems(buildActionItemsParams{
						selected:  selected,
						prs:       loadedPRs,
						conn:      loadedConn,
						prsLoaded: prsLoaded,
					})
					return components.NewSelectList(components.NewSelectListParams{Title: "Action", Items: items})
				},
				Summary: func(m any) string {
					sl, ok := m.(components.SelectListModel)
					if !ok {
						return ""
					}
					return sl.Value()
				},
			},
		},
		InitCmd: worktreepicker.PRLoadCmd(func() ([]domain.PRInfo, domain.GHConnection) {
			return shared.LoadPRs(params.projectDir)
		}),
		Loading:     true,
		LoadingText: worktreepicker.LoadingPRsText,
		OnMsg: func(w *components.WizardModel, msg tea.Msg) (tea.Cmd, bool) {
			if cmd, handled := worktreerefresh.Handle(worktreerefresh.HandleParams{
				Wizard:     w,
				Msg:        msg,
				ListParams: listParams,
				Holder:     holder,
			}); handled {
				return cmd, true
			}
			loaded, ok := msg.(worktreepicker.PRsLoadedMsg)
			if !ok {
				return nil, false
			}
			loadedPRs = loaded.PRs
			loadedConn = loaded.Conn
			prsLoaded = true
			badges := worktreepicker.BadgesByValue(worktreepicker.BadgesByValueParams{
				Statuses:     *holder,
				PRs:          loaded.PRs,
				Services:     params.services,
				ActiveBranch: params.activeBranch,
			})
			w.UpdateStepModel(0, func(model any) any {
				sl, ok := model.(components.SelectListModel)
				if !ok {
					return model
				}
				sl.SetBadges(badges)
				return sl
			})
			w.SetLoading(false)
			w.SetBanner(worktreepicker.GHBanner(loaded.Conn))
			return nil, true
		},
	})

	finalModel, err := tea.NewProgram(wiz).Run()
	if err != nil {
		return domain.WorktreeStatus{}, "", nil, fmt.Errorf("wizard: %w", err)
	}

	final, ok := finalModel.(components.WizardModel)
	if !ok || final.Aborted() {
		return domain.WorktreeStatus{}, "", nil, domain.ErrUserAborted
	}

	wtSL, ok := final.Steps()[0].Model.(components.SelectListModel)
	if !ok {
		return domain.WorktreeStatus{}, "", nil, fmt.Errorf("unexpected model type for worktree step")
	}

	idx, err := strconv.Atoi(wtSL.Value())
	if err != nil {
		return domain.WorktreeStatus{}, "", nil, fmt.Errorf("parse worktree index: %w", err)
	}

	actionSL, ok := final.Steps()[1].Model.(components.SelectListModel)
	if !ok {
		return domain.WorktreeStatus{}, "", nil, fmt.Errorf("unexpected model type for action step")
	}

	return statuses[idx], actionSL.Value(), loadedPRs, nil
}

// selectedWorktree resolves the worktree chosen in the wizard's first step.
func selectedWorktree(prev []components.Step, statuses []domain.WorktreeStatus) domain.WorktreeStatus {
	if len(prev) == 0 {
		return domain.WorktreeStatus{}
	}
	sl, ok := prev[0].Model.(components.SelectListModel)
	if !ok {
		return domain.WorktreeStatus{}
	}
	idx, err := strconv.Atoi(sl.Value())
	if err != nil || idx < 0 || idx >= len(statuses) {
		return domain.WorktreeStatus{}
	}
	return statuses[idx]
}

// buildActionItemsParams holds the inputs for buildActionItems.
type buildActionItemsParams struct {
	selected  domain.WorktreeStatus
	prs       []domain.PRInfo
	conn      domain.GHConnection
	prsLoaded bool
}

// buildActionItems builds the action menu for a worktree. While PRs are still
// loading, the Open PR entry stays enabled optimistically (the action resolves
// the URL per-branch at execution time); once loaded, it is disabled when the
// branch has no open PR or the GitHub CLI is unavailable.
func buildActionItems(params buildActionItemsParams) []components.SelectItem {
	openPRDisabled := false
	if params.prsLoaded {
		_, hasPR := findPRForBranch(params.prs, params.selected.Branch)
		openPRDisabled = !hasPR || params.conn != domain.GHConnectionOK
	}

	return []components.SelectItem{
		{Label: "Go (cd to worktree)", Value: lsActionGo},
		{Label: "Open PR", Value: lsActionOpenPR, Disabled: openPRDisabled},
		{Separator: true},
		{Label: "Start profile", Value: lsActionServicesUp},
		{Label: "Stop profile", Value: lsActionServicesDown},
		{Label: "View logs", Value: lsActionLogs},
		{Separator: true},
		{Label: "Clean (delete worktree)", Value: lsActionClean, Danger: true},
	}
}

func executeWorktreeAction(cmd *cobra.Command, action string, selected domain.WorktreeStatus, prs []domain.PRInfo, result shared.ConfigResult) error {
	bin, err := os.Executable()
	if err != nil {
		return err
	}

	switch action {
	case lsActionGo:
		goFile := os.Getenv(domain.EnvGoFile)
		if goFile != "" {
			return os.WriteFile(goFile, []byte(selected.Path), 0o644)
		}
		fmt.Println(selected.Path)
		return nil

	case lsActionOpenPR:
		if pr, ok := findPRForBranch(prs, selected.Branch); ok {
			return exec.Command("open", pr.URL).Run()
		}
		// PRs may not have finished streaming when the action menu was built;
		// resolve the URL for this branch directly.
		found, _, url := ghservice.HasOpenPR(ghservice.HasOpenPRParams{
			ProjectDir: result.ProjectDir,
			Branch:     selected.Branch,
		})
		if !found {
			return nil
		}
		return exec.Command("open", url).Run()

	case lsActionServicesUp:
		cmd := exec.Command(bin, domain.CmdRun, domain.CmdUp)
		cmd.Dir = selected.Path
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()

	case lsActionServicesDown:
		cmd := exec.Command(bin, domain.CmdRun, domain.CmdDown)
		cmd.Dir = selected.Path
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()

	case lsActionLogs:
		cmd := exec.Command(bin, domain.CmdRun, domain.CmdLogs)
		cmd.Dir = selected.Path
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()

	case lsActionClean:
		cmd := exec.Command(bin, domain.CmdClean, selected.Branch)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	return nil
}
