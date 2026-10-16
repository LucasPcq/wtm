package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
	"github.com/LucasPcq/wtm/internal/surface/cli/render"
)

const (
	createShort = "Create one or more worktrees"
	createLong  = "Create one or more git worktrees with env provisioning, metadata, and hooks.\n" +
		"Several branches are created one after the other from the same source; a failure\n" +
		"does not stop the others, and the run ends with what was created and what failed.\n" +
		"A branch that already exists locally is checked out as-is, keeping its commits;\n" +
		"its parent can't be inferred, so --from then names the branch recorded for `wtm sync`.\n" +
		"A branch only origin has is created from it, tracking it.\n" +
		"When run.toml declares jobs, a branch whose derived name a live worktree or another\n" +
		"branch of the run already carries (feat.x next to feat/x) is refused.\n" +
		"Without arguments, the form asks for the branches: tab adds another, enter continues."
	createExample = `  # Answer the form: branches, source, env strategy, isolation
  wtm create

  # Three worktrees from the base branch, no prompts
  wtm create feat/login feat/billing fix/header --yes

  # A stacked branch on top of feat/login
  wtm create feat/login-ui --from feat/login --yes

  # See the plan first, then apply exactly that plan
  wtm create feat/login --dry-run
  wtm create feat/login --expect-fingerprint <hash>

  # For a script or an agent: idempotent, with the v2 JSON envelope
  wtm create feat/login --if-not-exists --output json`

	createPathFastForward = "fast_forward"
	createFlagFastForward = "ff"
)

const (
	rowBranches    = "Branches"
	rowFrom        = "From"
	rowParent      = "Parent"
	rowEnv         = "Env"
	rowIsolation   = "Isolation"
	rowFastForward = "Fast-forward"
	rowRemember    = "Remember"
	existingSuffix = " (existing)"
	resumesFmt     = "%s (resumes %s)"
	openSuffix     = " (already open)"
	actionCreate   = "create"
	actionCheckOut = "check_out"
	actionResume   = "resume"
	actionOpen     = "unchanged"
)

type binder func(name string, doc Doc, options ...Option) *cobra.Command

func newCreateCmd(bind binder) *cobra.Command {
	cmd := bind(domain.CmdCreate, Doc{Use: domain.CmdCreate + " [branch...]", Short: createShort, Long: createLong, Example: createExample},
		Positional("branches"),
		Flag(FlagParams{Path: createPathFastForward, Name: createFlagFastForward}),
		Detail(createDetail{}),
	)
	cmd.GroupID = domain.CmdGroupWorktrees
	return cmd
}

type createDetail struct{}

type createPlan struct {
	Branches    []createPlanned `json:"branches"`
	From        string          `json:"from"`
	EnvFrom     string          `json:"env_from"`
	Isolation   string          `json:"isolation"`
	FastForward []string        `json:"fast_forward"`
	Remember    []string        `json:"remember"`
}

type createPlanned struct {
	Branch   string `json:"branch"`
	Action   string `json:"action"`
	Upstream string `json:"upstream"`
}

func (createDetail) Plan(detail json.RawMessage) []styles.PlanRow {
	var plan createPlan
	if json.Unmarshal(detail, &plan) != nil {
		return render.GenericDetail{}.Plan(detail)
	}
	rows := []styles.PlanRow{
		{Label: rowBranches, Values: plannedBranches(plan.Branches)},
		{Label: sourceRow(plan.Branches), Values: []string{plan.From}},
		{Label: rowEnv, Values: []string{plan.EnvFrom}},
	}
	if plan.Isolation != "" {
		rows = append(rows, styles.PlanRow{Label: rowIsolation, Values: []string{plan.Isolation}})
	}
	if len(plan.FastForward) > 0 {
		rows = append(rows, styles.PlanRow{Label: rowFastForward, Values: plan.FastForward})
	}
	if len(plan.Remember) > 0 {
		rows = append(rows, styles.PlanRow{Label: rowRemember, Values: plan.Remember})
	}
	return rows
}

func plannedBranches(branches []createPlanned) []string {
	values := make([]string, 0, len(branches))
	for _, planned := range branches {
		values = append(values, plannedBranch(planned))
	}
	return values
}

func plannedBranch(planned createPlanned) string {
	switch planned.Action {
	case actionCheckOut:
		return planned.Branch + existingSuffix
	case actionResume:
		return fmt.Sprintf(resumesFmt, planned.Branch, planned.Upstream)
	case actionOpen:
		return planned.Branch + openSuffix
	}
	return planned.Branch
}

func sourceRow(branches []createPlanned) string {
	for _, planned := range branches {
		if planned.Action == actionCreate {
			return rowFrom
		}
	}
	return rowParent
}

type createItem struct {
	Path           string             `json:"path"`
	ExistingBranch bool               `json:"existing_branch"`
	OriginAhead    int                `json:"origin_ahead"`
	OriginBehind   int                `json:"origin_behind"`
	EnvPorts       domain.EnvPortPlan `json:"env_ports"`
}

func (createDetail) Item(w io.Writer, item kernel.Item[json.RawMessage]) {
	var created createItem
	if json.Unmarshal(item.Detail, &created) != nil {
		return
	}
	render.InfoLine(w, item.Subject, shownPath(created.Path))
	if note, warns := reusedNote(item.Subject, created); warns {
		render.Warning(w, note)
	}
	if anomalies := rules.EnvPortAnomalyLines(created.EnvPorts); len(anomalies) > 0 {
		render.Section(w, domain.EnvPortAnomaliesTitle, anomalies)
	}
	render.NextStep(w, render.NextStepParams{Command: fmt.Sprintf(domain.GoCommandFmt, item.Subject)})
}

func reusedNote(branch string, created createItem) (string, bool) {
	if !created.ExistingBranch {
		return "", false
	}
	switch rules.ClassifyDivergence(created.OriginAhead, created.OriginBehind) {
	case domain.DivergenceBehind:
		return fmt.Sprintf(domain.BranchReusedBehindWarning, branch, created.OriginBehind), true
	case domain.DivergenceDiverged:
		return fmt.Sprintf(domain.BranchReusedDivergedWarning, branch, created.OriginAhead, created.OriginBehind), true
	case domain.DivergenceUpToDate, domain.DivergenceAhead, domain.DivergenceUnknown:
	}
	return "", false
}

func shownPath(path string) string {
	cwd, err := os.Getwd()
	if err != nil {
		return path
	}
	relative, err := filepath.Rel(cwd, path)
	if err != nil {
		return path
	}
	return relative
}
