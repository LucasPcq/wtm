package initcmd

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/detect"
	"github.com/LucasPcq/wtm/internal/tui/components"
	initwizard "github.com/LucasPcq/wtm/internal/tui/inittui"
)

// parseSections validates and de-duplicates the --only values (CSV or repeated),
// preserving order. Returns ExitCodeUsage-wrapped error on an unknown section.
func parseSections(raw []string) ([]string, error) {
	valid := map[string]bool{
		domain.SectionWorktrees: true,
		domain.SectionEnv:       true,
		domain.SectionHooks:     true,
	}
	seen := map[string]bool{}
	var sections []string
	for _, entry := range raw {
		for _, name := range strings.Split(entry, ",") {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			if name == domain.SectionServices {
				return nil, fmt.Errorf("services moved to a dedicated command — run `wtm run init` (experimental) to configure them")
			}
			if !valid[name] {
				return nil, fmt.Errorf("unknown section %q for --%s (valid: %s, %s, %s)",
					name, domain.FlagOnly, domain.SectionWorktrees, domain.SectionEnv, domain.SectionHooks)
			}
			if !seen[name] {
				seen[name] = true
				sections = append(sections, name)
			}
		}
	}
	return sections, nil
}

// runReinit re-runs init for the requested sections only, regenerating each
// section cleanly. config.toml sections preserve all untouched values; run.toml
// regenerates jobs while preserving profiles.
func runReinit(cmd *cobra.Command, dir, stateDir string, sections []string) error {
	if !detect.ProjectConfigExists(stateDir) {
		return fmt.Errorf("no wtm config found — run `wtm init` first: %w", domain.ErrConfigNotFound)
	}

	detection := detect.ProjectEnvironment(dir)

	nonInteractive, _ := cmd.Flags().GetBool(domain.FlagNonInteractive)

	var answers domain.InitProjectAnswers
	if nonInteractive {
		built, err := buildReinitAnswers(cmd, stateDir, detection)
		if err != nil {
			return err
		}
		answers = built
	} else {
		prefill, err := buildPrefill(stateDir)
		if err != nil {
			return err
		}
		// The re-init confirmation is the wizard's final step (unless --yes), so Esc
		// on it returns to the section steps instead of aborting the whole flow.
		yes, _ := cmd.Flags().GetBool(domain.FlagYes)
		var confirm *components.NewConfirmParams
		if !yes {
			confirm = &components.NewConfirmParams{
				Title:       "Re-initialize " + strings.Join(sections, ", "),
				Description: "This regenerates the selected section(s) cleanly.",
				Warning:     reinitWarning(sections),
			}
		}
		wizardAnswers, err := initwizard.RunSectionWizard(initwizard.SectionWizardParams{
			ProjectDir: dir,
			Sections:   sections,
			Detection:  detection,
			Prefill:    prefill,
			Confirm:    confirm,
		})
		if errors.Is(err, domain.ErrUserAborted) {
			output.Frame(cmd.OutOrStdout(), func(w io.Writer) {
				output.Message(w, "Aborted.")
			})
			return nil
		}
		if err != nil {
			return err
		}
		answers = wizardAnswers
	}

	if !contains(sections, domain.SectionWorktrees) && !contains(sections, domain.SectionEnv) && !contains(sections, domain.SectionHooks) {
		return nil
	}

	var applyErr error
	output.Frame(cmd.OutOrStdout(), func(w io.Writer) {
		applyErr = applyConfigReinit(applyReinitParams{
			Out:      w,
			StateDir: stateDir,
			Sections: sections,
			Answers:  answers,
		})
	})
	return applyErr
}

// buildPrefill snapshots the current config so the interactive re-init wizard
// pre-selects what's already configured.
func buildPrefill(stateDir string) (*initwizard.SectionPrefill, error) {
	cfg, err := config.LoadProjectRaw(stateDir)
	if err != nil {
		return nil, fmt.Errorf("load project config: %w", err)
	}
	return &initwizard.SectionPrefill{
		BaseBranch:  cfg.Worktrees.BaseBranch,
		EnvStrategy: string(cfg.Env.Strategy),
		EnvTargets:  toSet(rules.EnvTargets(cfg.Env.Files)),
		OnCreate:    cfg.Hooks.OnCreate,
		OnClean:     cfg.Hooks.OnClean,
	}, nil
}

func toSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, v := range values {
		set[v] = true
	}
	return set
}

// buildReinitAnswers resolves answers for the non-interactive path. Scalar
// values (base branch, env strategy, install command) keep their current config
// value unless a flag overrides them; the detected lists (env files, docker,
// scripts, monorepo) are regenerated from detection. NonInteractive is left
// false so an unresolved base branch falls back to a default rather than erroring.
func buildReinitAnswers(cmd *cobra.Command, stateDir string, detection domain.InitDetectionResult) (domain.InitProjectAnswers, error) {
	cfg, err := config.LoadProjectRaw(stateDir)
	if err != nil {
		return domain.InitProjectAnswers{}, fmt.Errorf("load project config: %w", err)
	}

	baseBranch, _ := cmd.Flags().GetString(domain.FlagBaseBranch)
	if baseBranch == "" {
		baseBranch = cfg.Worktrees.BaseBranch
	}
	envStrategy, _ := cmd.Flags().GetString(domain.FlagEnvStrategy)
	if envStrategy == "" {
		envStrategy = string(cfg.Env.Strategy)
	}
	installCommand, _ := cmd.Flags().GetString(domain.FlagInstallCommand)
	if installCommand == "" {
		installCommand = rules.InstallCommandFromHooks(cfg.Hooks.OnCreate)
	}
	cleanCommand, _ := cmd.Flags().GetString(domain.FlagCleanCommand)

	answers, err := rules.BuildProjectAnswers(rules.InitProjectFlags{
		BaseBranch:     baseBranch,
		EnvStrategy:    envStrategy,
		InstallCommand: installCommand,
		CleanCommand:   cleanCommand,
	}, detection)
	if err != nil {
		return domain.InitProjectAnswers{}, err
	}

	// on_clean has no single-command reverse like the install command, so preserve
	// the existing list when --clean-command was not provided.
	if cleanCommand == "" {
		answers.OnClean = cfg.Hooks.OnClean
	}

	return answers, nil
}

// applyConfigReinit rewrites config.toml, updating only the requested sections
// and preserving every other section's current values.
type applyReinitParams struct {
	Out      io.Writer
	StateDir string
	Sections []string
	Answers  domain.InitProjectAnswers
}

func applyConfigReinit(params applyReinitParams) error {
	stateDir, sections, answers := params.StateDir, params.Sections, params.Answers
	cfg, err := config.LoadProjectRaw(stateDir)
	if err != nil {
		return fmt.Errorf("load project config: %w", err)
	}

	if contains(sections, domain.SectionWorktrees) {
		cfg.Worktrees.BaseBranch = answers.BaseBranch
	}
	if contains(sections, domain.SectionEnv) {
		cfg.Env.Strategy = answers.EnvStrategy
		cfg.Env.Files = answers.EnvFiles
	}
	if contains(sections, domain.SectionHooks) {
		cfg.Hooks.OnCreate = answers.OnCreate
		cfg.Hooks.OnClean = answers.OnClean
	}

	if err := config.WriteProjectConfig(config.WriteProjectConfigParams{StateDir: stateDir, Config: cfg}); err != nil {
		return fmt.Errorf("write project config: %w", err)
	}

	output.Success(params.Out, "Rewrote config.toml")
	return nil
}

// reinitWarning describes what each requested section's regeneration will touch.
func reinitWarning(sections []string) string {
	var lines []string
	for _, s := range sections {
		switch s {
		case domain.SectionWorktrees:
			lines = append(lines, "config.toml [worktrees] base_branch will be rewritten")
		case domain.SectionEnv:
			lines = append(lines, "config.toml [env] will be rewritten")
		case domain.SectionHooks:
			lines = append(lines, "config.toml [hooks] will be rewritten")
		}
	}
	return strings.Join(lines, "; ")
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}
