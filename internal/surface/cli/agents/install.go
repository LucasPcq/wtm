package agents

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/service/detect"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

const (
	agentActionCreated   = "created"
	agentActionUpdated   = "updated"
	agentActionUnchanged = "unchanged"
	agentActionSkipped   = "skipped"
)

type agentInstallResult struct {
	Kind   domain.AgentKind `json:"kind"`
	Path   string           `json:"path"`
	Action string           `json:"action"`
	Reason string           `json:"reason,omitempty"`
}

func newInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the using-wtm skill into .claude / .cursor",
		Long:  "Detects which skill destinations exist (project and home-level .claude and .cursor)\nand installs the using-wtm skill into the ones you pick.",
		Example: `  wtm agents install

  # Every detected destination, no questions
  wtm agents install --yes

  # Also create the ones that don't exist yet, and report as JSON
  wtm agents install --all --yes --output json`,
		RunE: runInstall,
	}
	cmd.Flags().Bool(domain.FlagYes, false, "Non-interactive: install into every detected destination")
	cmd.Flags().Bool(domain.FlagAll, false, "Include destinations that don't yet exist (creates skill dirs)")
	shared.AddOutputFlag(cmd)
	return cmd
}

func runInstall(cmd *cobra.Command, _ []string) error {
	projectDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	yes, _ := cmd.Flags().GetBool(domain.FlagYes)
	all, _ := cmd.Flags().GetBool(domain.FlagAll)
	format, _ := cmd.Flags().GetString(domain.FlagOutput)

	targets := detect.AgentTargets(projectDir)

	selected, err := selectAgentTargets(cmd, targets, selectParams{
		Yes:        yes,
		All:        all,
		JSON:       format == domain.OutputJSON,
		IsTerminal: term.IsTerminal(int(os.Stdin.Fd())),
	})
	if errors.Is(err, domain.ErrUserAborted) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(selected) == 0 {
		if format == domain.OutputJSON {
			return writeAgentsJSON(cmd.OutOrStdout(), nil)
		}
		output.Frame(cmd.OutOrStdout(), func(w io.Writer) {
			output.Unchanged(w, "No destinations selected.")
		})
		return nil
	}

	results := make([]agentInstallResult, 0, len(selected))
	for _, t := range selected {
		results = append(results, applyAgentTarget(t))
	}

	if format == domain.OutputJSON {
		return writeAgentsJSON(cmd.OutOrStdout(), results)
	}
	printAgentResults(cmd.OutOrStdout(), results)
	return nil
}

type selectParams struct {
	Yes        bool
	All        bool
	JSON       bool
	IsTerminal bool
}

func selectAgentTargets(cmd *cobra.Command, targets []domain.AgentTarget, p selectParams) ([]domain.AgentTarget, error) {
	if p.Yes || p.JSON || !p.IsTerminal {
		out := make([]domain.AgentTarget, 0, len(targets))
		for _, t := range targets {
			if t.Exists || p.All {
				out = append(out, t)
			}
		}
		return out, nil
	}

	items := make([]components.MultiSelectItem, 0, len(targets))
	for _, t := range targets {
		items = append(items, components.MultiSelectItem{
			Label:    agentTargetLabel(t),
			Value:    string(t.Kind),
			Selected: t.Exists,
		})
	}
	_ = cmd
	selected, err := runAgentsMultiSelect(items)
	if err != nil {
		return nil, err
	}

	chosen := make(map[string]struct{}, len(selected))
	for _, v := range selected {
		chosen[v] = struct{}{}
	}
	out := make([]domain.AgentTarget, 0, len(targets))
	for _, t := range targets {
		if _, ok := chosen[string(t.Kind)]; ok {
			out = append(out, t)
		}
	}
	return out, nil
}

func agentTargetLabel(t domain.AgentTarget) string {
	suffix := "will be created"
	if t.Exists {
		suffix = "detected"
	}
	scope := "project"
	if t.IsGlobal {
		scope = "global"
	}
	switch t.Kind {
	case domain.AgentKindClaudeProject, domain.AgentKindClaudeGlobal:
		return fmt.Sprintf("Claude (%s) — %s (%s)", scope, t.Path, suffix)
	case domain.AgentKindCursorProject, domain.AgentKindCursorGlobal:
		return fmt.Sprintf("Cursor (%s) — %s (%s)", scope, t.Path, suffix)
	}
	return t.Path
}

func applyAgentTarget(t domain.AgentTarget) agentInstallResult {
	return writeSkill(t)
}

// writeSkill installs the skill directory holding t.Path: every shipped file,
// and the removal of a reference this wtm no longer ships. Nothing else in the
// directory is touched: it may hold the user's own notes.
func writeSkill(t domain.AgentTarget) agentInstallResult {
	dir := filepath.Dir(t.Path)
	_, statErr := os.Stat(t.Path)
	existed := statErr == nil
	if statErr != nil && !os.IsNotExist(statErr) {
		return agentInstallResult{Kind: t.Kind, Path: t.Path, Action: agentActionSkipped, Reason: statErr.Error()}
	}

	files := skillFiles()
	changed := false
	for name, content := range files {
		wrote, err := writeIfDifferent(filepath.Join(dir, filepath.FromSlash(name)), content)
		if err != nil {
			return agentInstallResult{Kind: t.Kind, Path: t.Path, Action: agentActionSkipped, Reason: err.Error()}
		}
		changed = changed || wrote
	}
	removed, err := removeStaleReferences(dir, files)
	if err != nil {
		return agentInstallResult{Kind: t.Kind, Path: t.Path, Action: agentActionSkipped, Reason: err.Error()}
	}

	switch {
	case !existed:
		return agentInstallResult{Kind: t.Kind, Path: t.Path, Action: agentActionCreated}
	case changed || removed:
		return agentInstallResult{Kind: t.Kind, Path: t.Path, Action: agentActionUpdated}
	default:
		return agentInstallResult{Kind: t.Kind, Path: t.Path, Action: agentActionUnchanged}
	}
}

func writeIfDifferent(file, content string) (bool, error) {
	existing, err := os.ReadFile(file)
	if err == nil && string(existing) == content {
		return false, nil
	}
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return false, err
	}
	return true, os.WriteFile(file, []byte(content), 0o644)
}

func removeStaleReferences(dir string, shipped map[string]string) (bool, error) {
	entries, err := os.ReadDir(filepath.Join(dir, skillReferencesDir))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	removed := false
	for _, entry := range entries {
		name := skillReferencesDir + "/" + entry.Name()
		if _, ok := shipped[name]; ok || entry.IsDir() {
			continue
		}
		if err := os.Remove(filepath.Join(dir, skillReferencesDir, entry.Name())); err != nil {
			return removed, err
		}
		removed = true
	}
	return removed, nil
}

func runAgentsMultiSelect(items []components.MultiSelectItem) ([]string, error) {
	ms := components.NewMultiSelect(components.NewMultiSelectParams{
		Title:       "Install wtm usage guide into",
		Description: "Space to toggle, enter to confirm. Detected destinations are pre-selected.",
		Items:       items,
	})

	wiz := components.NewWizard([]components.Step{{
		Name:  "Destinations",
		Model: ms,
	}})

	finalModel, err := tea.NewProgram(wiz).Run()
	if err != nil {
		return nil, fmt.Errorf("select destinations: %w", err)
	}
	final, ok := finalModel.(components.WizardModel)
	if !ok || final.Aborted() {
		return nil, domain.ErrUserAborted
	}
	step, ok := final.Steps()[0].Model.(components.MultiSelectModel)
	if !ok {
		return nil, errors.New("unexpected model")
	}
	return step.Values(), nil
}

func writeAgentsJSON(w io.Writer, results []agentInstallResult) error {
	if results == nil {
		results = []agentInstallResult{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(results)
}

func printAgentResults(dest io.Writer, results []agentInstallResult) {
	output.Frame(dest, func(w io.Writer) {
		for _, r := range results {
			switch r.Action {
			case agentActionCreated:
				output.Success(w, fmt.Sprintf("Created %s", r.Path))
			case agentActionUpdated:
				output.Update(w, fmt.Sprintf("Updated %s", r.Path))
			case agentActionUnchanged:
				output.Unchanged(w, fmt.Sprintf("Up to date %s", r.Path))
			case agentActionSkipped:
				if r.Reason != "" {
					output.Warning(w, fmt.Sprintf("Skipped %s — %s", r.Path, r.Reason))
				} else {
					output.Warning(w, fmt.Sprintf("Skipped %s", r.Path))
				}
			}
		}
	})
}
