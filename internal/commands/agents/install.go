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
		RunE:  runInstall,
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
			output.Message(w, "No destinations selected.")
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
	return writeSkillFile(t)
}

func writeSkillFile(t domain.AgentTarget) agentInstallResult {
	content := renderSkillMarkdown()

	existing, err := os.ReadFile(t.Path)
	if err == nil {
		if string(existing) == content {
			return agentInstallResult{Kind: t.Kind, Path: t.Path, Action: agentActionUnchanged}
		}
		if writeErr := os.WriteFile(t.Path, []byte(content), 0o644); writeErr != nil {
			return agentInstallResult{Kind: t.Kind, Path: t.Path, Action: agentActionSkipped, Reason: writeErr.Error()}
		}
		return agentInstallResult{Kind: t.Kind, Path: t.Path, Action: agentActionUpdated}
	}
	if !os.IsNotExist(err) {
		return agentInstallResult{Kind: t.Kind, Path: t.Path, Action: agentActionSkipped, Reason: err.Error()}
	}
	if err := os.MkdirAll(filepath.Dir(t.Path), 0o755); err != nil {
		return agentInstallResult{Kind: t.Kind, Path: t.Path, Action: agentActionSkipped, Reason: err.Error()}
	}
	if err := os.WriteFile(t.Path, []byte(content), 0o644); err != nil {
		return agentInstallResult{Kind: t.Kind, Path: t.Path, Action: agentActionSkipped, Reason: err.Error()}
	}
	return agentInstallResult{Kind: t.Kind, Path: t.Path, Action: agentActionCreated}
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

func printAgentResults(w io.Writer, results []agentInstallResult) {
	output.Frame(w, func(w io.Writer) {
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
