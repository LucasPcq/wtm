package rules

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// DisplayPathParams holds the inputs for DisplayPath.
type DisplayPathParams struct {
	// Base is the directory the result is made relative to.
	Base string
	// Target is the path to render.
	Target string
}

// DisplayPath renders Target relative to Base for readable output, falling back to
// Target unchanged when it lies outside Base or cannot be made relative. Lexical
// only: no filesystem access.
func DisplayPath(p DisplayPathParams) string {
	rel, err := filepath.Rel(p.Base, p.Target)
	if err != nil || strings.HasPrefix(rel, "..") {
		return p.Target
	}
	return rel
}

// InitGlobalFlags holds the raw --shell input for non-interactive init.
type InitGlobalFlags struct {
	Shell string
}

// InitProjectFlags holds the raw project-config inputs for non-interactive init.
// NonInteractive makes unresolved required values fail rather than silently
// falling back to a constant default. The Skip* flags opt out of optional
// sections, mirroring the wizard skip key. Services are no longer part of the
// global init — they are configured by the dedicated `wtm run init` command.
type InitProjectFlags struct {
	BasePath       string
	BaseBranch     string
	EnvStrategy    string
	InstallCommand string
	CleanCommand   string
	NonInteractive bool
	SkipEnv        bool
	SkipHooks      bool
	SkipClean      bool
}

// BuildGlobalAnswers resolves global config from flags, falling back to the
// constant defaults. Returns ErrInvalidShellType when a provided value is not a
// known enum member.
func BuildGlobalAnswers(flags InitGlobalFlags) (domain.InitGlobalAnswers, error) {
	shell := domain.DefaultShell
	if flags.Shell != "" {
		shell = domain.ShellType(flags.Shell)
	}
	if err := ValidateShellType(shell); err != nil {
		return domain.InitGlobalAnswers{}, err
	}

	return domain.InitGlobalAnswers{Shell: shell}, nil
}

// BuildProjectAnswers resolves project config from flags and auto-detection,
// mirroring the wizard defaults: flags win, then detection, then constants.
// Conditional multi-selects (env files, package scripts, docker compose) take
// every detected value, matching the wizard's pre-selection. The on_create
// install hooks come from BuildInstallHooks, which decides from the detected
// monorepo layout whether a single root install suffices.
func BuildProjectAnswers(flags InitProjectFlags, detection domain.InitDetectionResult) (domain.InitProjectAnswers, error) {
	basePath := flags.BasePath
	if basePath == "" {
		basePath = domain.DefaultBasePath
	}

	baseBranch := flags.BaseBranch
	if baseBranch == "" {
		baseBranch = detection.BaseBranch
	}
	if baseBranch == "" {
		if flags.NonInteractive {
			return domain.InitProjectAnswers{}, fmt.Errorf("base branch could not be detected — pass --%s", domain.FlagBaseBranch)
		}
		baseBranch = domain.DefaultBaseBranch
	}

	answers := domain.InitProjectAnswers{
		BasePath:   basePath,
		BaseBranch: baseBranch,
	}

	if flags.SkipEnv {
		answers.SkipEnv = true
	} else {
		envStrategy := domain.DefaultEnvStrategy
		if flags.EnvStrategy != "" {
			envStrategy = domain.EnvStrategy(flags.EnvStrategy)
		}
		if err := ValidateEnvStrategy(envStrategy); err != nil {
			return domain.InitProjectAnswers{}, err
		}
		answers.EnvStrategy = envStrategy
		answers.EnvFiles = detection.EnvFiles
	}

	if flags.SkipHooks {
		answers.SkipHooks = true
	} else {
		installCommand := flags.InstallCommand
		if installCommand == "" {
			installCommand = detection.InstallCommand
		}
		answers.OnCreate = BuildInstallHooks(InstallHooksParams{
			RootCommand: installCommand,
			Kind:        detection.Monorepo.Kind,
			SubProjects: detection.Monorepo.SubProjects,
		})
	}

	// on_clean has no auto-detected seed (nothing to detect); it stays empty
	// unless --clean-command is provided.
	if flags.SkipClean {
		answers.SkipClean = true
	} else if flags.CleanCommand != "" {
		answers.OnClean = append(answers.OnClean, domain.HookCommand{Cmd: flags.CleanCommand})
	}

	return answers, nil
}

// InitGlobalRecapFields maps resolved global answers to the aligned label/value
// rows of the framed init recap. Pure: no I/O.
func InitGlobalRecapFields(answers domain.InitGlobalAnswers) []domain.RecapField {
	return []domain.RecapField{
		{Label: domain.InitRecapLabelShell, Value: string(answers.Shell)},
	}
}

// InitProjectRecapFields maps resolved project answers to the aligned label/value
// rows of the framed init recap. Opted-out sections render as "skipped"; on_create
// is omitted entirely when empty. Pure: no I/O.
func InitProjectRecapFields(answers domain.InitProjectAnswers) []domain.RecapField {
	fields := []domain.RecapField{
		{Label: domain.InitRecapLabelBasePath, Value: answers.BasePath},
		{Label: domain.InitRecapLabelBaseBranch, Value: answers.BaseBranch},
		{Label: domain.InitRecapLabelEnvStrategy, Value: initEnvValue(answers)},
	}
	if value := initHookValue(answers.SkipHooks, answers.OnCreate); value != "" {
		fields = append(fields, domain.RecapField{Label: domain.InitRecapLabelOnCreate, Value: value})
	}
	if value := initHookValue(answers.SkipClean, answers.OnClean); value != "" {
		fields = append(fields, domain.RecapField{Label: domain.InitRecapLabelOnClean, Value: value})
	}
	return fields
}

func initEnvValue(answers domain.InitProjectAnswers) string {
	if answers.SkipEnv {
		return domain.InitRecapValueSkippedTemplate
	}
	return string(answers.EnvStrategy)
}

// initHookValue renders a hook section for the recap: "skipped" when opted out,
// empty (omitted row) when unset, else the condensed HookSummary form.
func initHookValue(skip bool, hooks []domain.HookCommand) string {
	if skip {
		return domain.InitRecapValueSkipped
	}
	return HookSummary(hooks)
}

// HookSummary condenses a hook list to one line: "" when empty, the single
// command on its own, else the first command with a "(+N more)" suffix.
func HookSummary(hooks []domain.HookCommand) string {
	if len(hooks) == 0 {
		return ""
	}
	first := hooks[0].Cmd
	if len(hooks) == 1 {
		return first
	}
	return fmt.Sprintf(domain.InitRecapHookMoreFmt, first, len(hooks)-1)
}

// AutoServicesAnswers builds the services portion of InitProjectAnswers from
// detection alone — every detected docker-compose file and package script — for
// the non-interactive `wtm run init` path. The base config fields are left
// zero-valued: only the services fields feed BuildInitRunConfig.
func AutoServicesAnswers(detection domain.InitDetectionResult) domain.InitProjectAnswers {
	answers := domain.InitProjectAnswers{
		SelectedPackageScripts: detection.PackageScripts,
	}
	if len(detection.DockerComposeFiles) > 0 {
		answers.DockerComposeFiles = detection.DockerComposeFiles
		answers.DockerComposeCmd = detection.DockerComposeCmd
	}
	return answers
}
