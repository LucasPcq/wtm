package run

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/compose"
	"github.com/LucasPcq/wtm/internal/service/detect"
	envsvc "github.com/LucasPcq/wtm/internal/service/env"
	"github.com/LucasPcq/wtm/internal/service/proxy"
	"github.com/LucasPcq/wtm/internal/service/runconfig"
	"github.com/LucasPcq/wtm/internal/tui/components"
	initwizard "github.com/LucasPcq/wtm/internal/tui/inittui"
)

// newInitCmd creates the wtm run init subcommand — the dedicated entry point
// that configures the (experimental) run module, kept out of the global
// `wtm init` wizard so users who never touch `run` aren't bothered by it.
func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdInit,
		Short: "Configure the run module (services & tasks) for this repo",
		Long: "Set up run.toml by detecting docker-compose files and package.json scripts and turning\n" +
			"the selected ones into jobs.\n\n" +
			"In a TTY, opens a wizard to pick which ones to include; non-interactively (or piped),\n" +
			"auto-generates from detection. Re-running pre-fills every step from the existing\n" +
			"run.toml: what stays checked is kept, what you uncheck is removed along with the\n" +
			"profile entries and .env links naming it. Only jobs this wizard proposed are ever\n" +
			"removed — one added with `wtm run job add` is never listed, so never touched.\n" +
			"A non-interactive run asks nothing and removes nothing.\n\n" +
			"Ports declared in the selected compose files become per-worktree ports. A literal\n" +
			"host port (\"5432:5432\") binds the same port everywhere, so wtm offers to rewrite it\n" +
			"as \"${DB_PORT:-5432}:5432\" — the default keeps `docker compose up` working on its\n" +
			"own. Declining leaves the file untouched and declares no port for it.\n\n" +
			"The names those files pin absolutely get the same treatment. A container_name, or\n" +
			"a volume's or network's explicit name, is resolved by the Docker daemon rather than\n" +
			"by the compose project, so COMPOSE_PROJECT_NAME never reaches it and a second\n" +
			"worktree collides on it. wtm offers to front them with the project — a renamed\n" +
			"volume starts empty, its data staying under the name it used to carry.\n\n" +
			"In a monorepo, a root script that starts several apps at once is asked which\n" +
			"declared jobs it runs. wtm reads the directory a script sits in, never what its\n" +
			"command does: the relation is declared, and it is what keeps a runner from being\n" +
			"reported as a service that forgot its port — and from being started alongside one\n" +
			"of its own children.\n\n" +
			"Dev servers get theirs from the env files sitting next to their package.json —\n" +
			"a PORT (or *_PORT) entry in .env.local, .env, or a committed .env.example. A\n" +
			"service nothing was found for is offered anyway: declaring its port is what keeps\n" +
			"a second worktree from binding the same one.\n\n" +
			"wtm injects the variable, it never edits the command. When a command never\n" +
			"mentions the port it is given, the wizard offers it for editing on the spot\n" +
			"(`pnpm dev --port ${PORT}`) rather than reporting it once it is too late.\n\n" +
			"The mode those names are written in is asked too, because it is the one choice\n" +
			"with a consequence outside wtm: named urls are served by the run proxy, so they\n" +
			"answer while `wtm run` runs the job and not when you start it yourself. A project\n" +
			"whose author launches their own dev servers wants ports.\n\n" +
			"Every service that declares the port it listens on is then offered a name of its\n" +
			"own — <job>.<worktree>.<repo>.localhost, served by the proxy — so two worktrees\n" +
			"stop sharing a cookie jar. A port a job only dials (DB_PORT, REDIS_PORT) is never\n" +
			"offered: a name nothing answers under is worse than no name at all.\n\n" +
			domain.ExperimentalRunNotice,
		Args: cobra.NoArgs,
		RunE: runRunInit,
	}
	shared.AddNoPromptFlags(cmd, "Auto-generate from detection; never prompt")
	cmd.Flags().Bool(domain.FlagPatchCompose, false, "Rewrite the selected compose files' literal host ports and absolute names to read a variable")
	cmd.Flags().Bool(domain.FlagLinkEnv, false, "Link the .env keys holding a declared port, so each worktree gets its own")
	cmd.Flags().Bool(domain.FlagWritePortKeys, false, "Write each declared port into the job's .env and its template, so an app launched by hand reads the worktree's port")
	return cmd
}

func runRunInit(cmd *cobra.Command, _ []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	res, err := shared.LoadConfig(cmd, dir)
	if err != nil {
		return err
	}

	nonInteractive := shared.NoPrompt(cmd)
	patchCompose, _ := cmd.Flags().GetBool(domain.FlagPatchCompose)
	linkEnv, _ := cmd.Flags().GetBool(domain.FlagLinkEnv)
	writePortKeys, _ := cmd.Flags().GetBool(domain.FlagWritePortKeys)
	interactive := !nonInteractive && term.IsTerminal(int(os.Stdin.Fd()))

	var detection domain.InitDetectionResult
	var envScans map[string]domain.EnvPortScan
	_ = components.RunLoading(components.LoadingParams{
		Message: "Detecting services…",
		Animate: interactive,
		Work: func() error {
			detection = detect.ProjectEnvironment(res.ProjectDir)
			detection.ComposeScans = compose.ScanAll(compose.ScanAllParams{
				ProjectDir: res.ProjectDir,
				Files:      detection.DockerComposeFiles,
				Project:    filepath.Base(res.ProjectDir),
			})
			envScans = detect.ScanEnvPorts(detect.ScanEnvPortsParams{
				ProjectDir: res.ProjectDir,
				Files:      detection.EnvFiles,
			})
			return nil
		},
	})

	existing, err := runconfig.Load(res.StateDir)
	if err != nil {
		return fmt.Errorf("load run.toml: %w", err)
	}

	answers, err := resolveServicesAnswers(resolveServicesParams{
		Interactive:  interactive,
		ProjectDir:   res.ProjectDir,
		Detection:    detection,
		Existing:     existing,
		PatchCompose: patchCompose,
		EnvScans:     envScans,
		EnvLines: detect.EnvLines(detect.EnvPortCandidatesParams{
			ProjectDir: res.ProjectDir,
			Files:      res.Config.Project.Env.Files,
		}),
		EnvFiles: res.Config.Project.Env.Files,
	})
	if errors.Is(err, domain.ErrUserAborted) {
		return nil
	}
	if err != nil {
		return err
	}

	// A run that never put the scope question leaves what run.toml declares
	// standing: the pair (value, asked) again — emptied-and-asked withdraws,
	// not-asked keeps. Scans travel with the answers because naming the services
	// that stay in a file's job needs them.
	answers.Scans = detection.ComposeScans
	if !answers.ScopesAsked {
		answers.SharedServices = rules.SharedFromConfig(rules.SharedFromConfigParams{
			Existing: existing,
			Scans:    detection.ComposeScans,
			Files:    answers.DockerComposeFiles,
		})
	}

	plan := rules.PlanComposePorts(rules.PlanComposePortsParams{
		Scans: detection.ComposeScans,
		Files: answers.DockerComposeFiles,
		Patch: answers.PatchCompose,
	})
	namePlan := rules.PlanComposeNames(rules.PlanComposeNamesParams{
		Scans: detection.ComposeScans,
		Files: answers.DockerComposeFiles,
		Patch: answers.PatchCompose,
	})
	unverifiable := compose.VerifyAll(compose.VerifyAllParams{
		ProjectDir:  res.ProjectDir,
		ByFile:      plan.Patches,
		NamesByFile: namePlan.Patches,
	})
	namePatches := rules.ComposeNamesWithoutFiles(namePlan.Patches, rules.SortedComposeFiles(unverifiable))
	outcome := rules.ResolveDetectedPorts(rules.ResolveDetectedPortsParams{
		Answers:        answers,
		PackageManager: detection.PackageManager,
		Existing:       existing,
		Deselected: rules.DeselectedJobs(rules.DeselectedJobsParams{
			Existing:             existing,
			PackageManager:       detection.PackageManager,
			DetectedScripts:      detection.PackageScripts,
			SelectedScripts:      answers.SelectedPackageScripts,
			DetectedComposeFiles: detection.DockerComposeFiles,
			SelectedComposeFiles: answers.DockerComposeFiles,
			Asked:                answers.SelectionAsked,
		}),
		Plan:          plan,
		Unverifiable:  unverifiable,
		EnvScansByDir: envScans,
	})

	if !rules.IsRunInitialized(outcome.Config) {
		output.Frame(cmd.OutOrStdout(), func() {
			output.Message(cmd.OutOrStdout(), "No docker-compose files or package scripts detected — nothing to configure automatically.")
			output.Message(cmd.OutOrStdout(), "Add jobs by hand with `wtm run job add`, then group them with `wtm run profile add`.")
			output.Blank(cmd.OutOrStdout())
			output.Message(cmd.OutOrStdout(), domain.ExperimentalRunNotice)
		})
		return nil
	}

	// The wizard's two composition steps outrank detection. A step that never
	// ran leaves the proposal standing — a profile is what makes `run up` start
	// something rather than everything — but one that ran and was emptied is an
	// answer, and reinstating it would undo the user's own gesture.
	if !answers.ProfilesAsked && len(answers.Profiles) == 0 {
		answers.Profiles = rules.ProposeProfiles(rules.ProposeProfilesParams{
			Config:   outcome.Config,
			Existing: existing.Profiles,
		})
	}
	outcome.Config = rules.ApplyInitAnswers(rules.ApplyInitAnswersParams{
		Config:          outcome.Config,
		Runners:         answers.Runners,
		Addressing:      answers.Addressing,
		AddressingAsked: answers.AddressingAsked,
		Ports:           answers.Ports,
		Profiles:        answers.Profiles,
		ProfilesAsked:   answers.ProfilesAsked,
		Cmds:            answers.Cmds,
		URLs:            answers.URLs,
		URLsAsked:       answers.URLsAsked,
		NewJobs:         outcome.Merge.Added,
	})

	// After the profiles are settled: the step re-proposes them from the config
	// on disk, so a lifted job inserted any earlier is discarded — and a profile
	// that no longer starts the database leaves every worktree addressing one
	// that was never created.
	outcome.Config = rules.JoinSharedProfiles(rules.JoinSharedProfilesParams{
		Config: outcome.Config,
		Shared: answers.SharedServices,
	})

	links := resolveEnvPortLinks(resolveEnvPortLinksParams{
		// The wizard already put the question as a step; asking again outside it
		// is the orphaned prompt this flow used to end on.
		Interactive: interactive && !answers.EnvLinksAsked,
		LinkEnv:     linkEnv || (answers.EnvLinksAsked && answers.LinkEnv),
		Declined:    answers.EnvLinksAsked && !answers.LinkEnv,
		ProjectDir:  res.ProjectDir,
		EnvFiles:    res.Config.Project.Env.Files,
		Config:      outcome.Config,
	})
	outcome.Config.EnvPorts = append(outcome.Config.EnvPorts, links...)

	// Writing a committed template is never inferred, exactly as the compose
	// patching is not: it takes the flag, or the wizard's route answer.
	var portKeys []domain.PortKeyWrite
	if writePortKeys || answers.PortRoutesAsked {
		portKeys = rules.PortKeyWrites(rules.PortKeyWritesParams{
			Config:     outcome.Config,
			Ports:      rules.PortRouteEnvPorts(answers),
			ScansByDir: envScans,
			EnvFiles:   res.Config.Project.Env.Files,
		})
	}
	writtenKeys, err := envsvc.WritePortKeys(envsvc.WritePortKeysParams{ProjectDir: res.ProjectDir, Writes: portKeys})
	if err != nil {
		return err
	}
	outcome.Config.EnvPorts = append(outcome.Config.EnvPorts, rules.PortKeyLinks(rules.PortKeyLinksParams{
		Writes:   portKeys,
		Existing: outcome.Config.EnvPorts,
	})...)

	// Last, once both tables are complete: a key an [[env]] link writes in full
	// has no port link, and the two are refused together at load — so a run that
	// only added the value link would write a config wtm then refuses to read.
	outcome.Config = rules.PruneEnvPortClashes(outcome.Config)

	// The rewrites come first: a compose templatized without run.toml behind it
	// keeps binding its defaults, while a run.toml declaring ports the compose
	// does not read would announce an isolation that is not there.
	if err := compose.PatchAll(compose.PatchAllParams{
		ProjectDir:  res.ProjectDir,
		ByFile:      outcome.Patches,
		NamesByFile: namePatches,
	}); err != nil {
		return err
	}
	if err := runconfig.Save(runconfig.SaveParams{StateDir: res.StateDir, Config: outcome.Config}); err != nil {
		return err
	}

	// Last of the writes: a provisioning target with no link behind it would
	// have wtm copying a file nothing in run.toml speaks about.
	addedTargets := rules.PortKeyTargets(rules.PortKeyTargetsParams{
		Writes:   portKeys,
		Existing: res.Config.Project.Env.Files,
	})
	if len(addedTargets) > 0 {
		project := res.Config.Project
		project.Env.Files = append(project.Env.Files, addedTargets...)
		if err := config.WriteProjectConfig(config.WriteProjectConfigParams{StateDir: res.StateDir, Config: project}); err != nil {
			return fmt.Errorf("add env targets: %w", err)
		}
	}
	reportedKeys := rules.PortKeysReported(rules.PortKeysReportedParams{
		Applied: writtenKeys,
		Writes:  portKeys,
		Targets: addedTargets,
	})

	// Re-read after the writes: the two reports below say what is still missing,
	// and the scan they were computed from predates the keys this run just wrote.
	envScans = detect.ScanEnvPorts(detect.ScanEnvPortsParams{
		ProjectDir: res.ProjectDir,
		Files:      detection.EnvFiles,
	})

	runPath := filepath.Join(res.StateDir, domain.RunFileName)
	output.Frame(cmd.OutOrStdout(), func() {
		output.Success(cmd.OutOrStdout(), fmt.Sprintf("Configured run module → %s", runPath))
		if len(outcome.Merge.Added) > 0 {
			output.Message(cmd.OutOrStdout(), fmt.Sprintf("Jobs added: %s", strings.Join(outcome.Merge.Added, ", ")))
		}
		if len(outcome.Removed) > 0 {
			output.Message(cmd.OutOrStdout(), fmt.Sprintf(domain.RunInitJobsRemovedFmt, strings.Join(outcome.Removed, ", ")))
		}
		if len(outcome.Merge.Skipped) > 0 {
			output.Message(cmd.OutOrStdout(), fmt.Sprintf("Already present (kept): %s", strings.Join(outcome.Merge.Skipped, ", ")))
		}
		output.DetectedPortsReport(cmd.OutOrStdout(), output.DetectedPortsReportParams{
			Patched:       outcome.Patches,
			Written:       outcome.Written,
			Withheld:      outcome.Withheld,
			JobsByFile:    composeJobsByFile(outcome.Config, answers.DockerComposeFiles),
			Dropped:       outcome.Dropped,
			Unreadable:    outcome.Unreadable,
			Changed:       outcome.Changed,
			Orphaned:      outcome.Orphaned,
			EnvWritten:    outcome.EnvWritten,
			EnvSources:    outcome.EnvSources,
			EnvUnreadable: outcome.EnvUnreadable,
		})
		output.ComposeNamesReport(cmd.OutOrStdout(), output.ComposeNamesReportParams{
			Patched:  namePatches,
			Withheld: namePlan.Withheld,
		})
		output.EnvPortLinksReport(cmd.OutOrStdout(), links, rules.EnvPortBases(outcome.Config))
		output.PortKeysReport(cmd.OutOrStdout(), reportedKeys)
		// Last, and alone in a frame: everything above is what the run did, this
		// is what it could not do without the reader.
		composeJobs := rules.ComposeJobsFor(rules.ComposeJobsParams{Config: outcome.Config, Files: answers.DockerComposeFiles})
		output.PortIsolationReport(cmd.OutOrStdout(), output.PortIsolationReportParams{
			Unported: rules.ServicesWithoutPorts(outcome.Config),
			Ignoring: rules.JobsMissingPortRef(rules.JobsMissingPortRefParams{
				Config: outcome.Config,
				Exempt: append(composeJobs,
					rules.JobsReadingTheirEnv(rules.JobsReadingTheirEnvParams{Config: outcome.Config, ScansByDir: envScans})...),
			}),
		})
		output.PortCommandOnlyReport(cmd.OutOrStdout(), rules.JobsIsolatedByCommand(rules.JobsIsolatedByCommandParams{
			Config:     outcome.Config,
			Exempt:     composeJobs,
			ScansByDir: envScans,
		}))
		proxyPort := rules.ProxyPort(res.Config.Global)
		if lines := rules.ProxyPortCollisionLines(rules.ProxyPortCollisions(rules.ProxyPortCollisionsParams{
			Config:    outcome.Config,
			ProxyPort: proxyPort,
		}), proxyPort); len(lines) > 0 {
			output.Callout(cmd.ErrOrStderr(), domain.ProxyPortCollisionTitle, lines)
		}
		if lines := rules.ProxyInstallHintLines(rules.ProxyInstallHintParams{
			Config:     outcome.Config,
			Status:     proxy.NewRedirector(proxy.RedirectorParams{}).Inspect(),
			ExampleURL: fmt.Sprintf(domain.ProxyURLFmt, domain.ProxyHostShape, proxyPort),
		}); len(lines) > 0 {
			output.Callout(cmd.ErrOrStderr(), domain.ProxyInstallHintTitle, lines)
		}
		// The main checkout is the one no command ever provisions, so it is the
		// one the addressing just chosen leaves behind.
		noticeAddressingDrift(cmd, res, res.ProjectDir)
		output.Blank(cmd.OutOrStdout())
		output.Message(cmd.OutOrStdout(), "Next: `wtm run up` to start · `wtm run job add` to add more")
		output.Blank(cmd.OutOrStdout())
		output.Message(cmd.OutOrStdout(), domain.ExperimentalRunNotice)
	})
	return nil
}

type resolveServicesParams struct {
	Interactive  bool
	ProjectDir   string
	Detection    domain.InitDetectionResult
	Existing     domain.RunConfig
	PatchCompose bool
	EnvScans     map[string]domain.EnvPortScan
	EnvLines     map[string][]domain.EnvLine
	EnvFiles     []domain.EnvFile
}

// resolveServicesAnswers gathers the services selection either from the wizard
// (interactive) or straight from detection (non-interactive). On a re-run the
// wizard is pre-filled with what run.toml already declares so the subsequent
// merge is additive rather than a fresh overwrite.
func resolveServicesAnswers(params resolveServicesParams) (domain.InitProjectAnswers, error) {
	if !params.Interactive {
		return rules.AutoServicesAnswers(rules.AutoServicesAnswersParams{
			Detection:    params.Detection,
			PatchCompose: params.PatchCompose,
		}), nil
	}

	var prefill *initwizard.SectionPrefill
	if rules.IsRunInitialized(params.Existing) {
		prefill = &initwizard.SectionPrefill{
			DockerFiles:   rules.DockerFilesConfigured(params.Existing, params.Detection.DockerComposeFiles),
			ScriptIndices: rules.ScriptsConfigured(params.Existing, params.Detection.PackageScripts, params.Detection.PackageManager),
		}
	}

	return initwizard.RunServicesWizard(initwizard.ServicesWizardParams{
		ProjectDir:   params.ProjectDir,
		Detection:    params.Detection,
		Existing:     params.Existing,
		Prefill:      prefill,
		PatchCompose: params.PatchCompose,
		EnvScans:     params.EnvScans,
		EnvLines:     params.EnvLines,
		EnvFiles:     params.EnvFiles,
	})
}

// composeJobsByFile turns a withheld port's fix into a command to paste rather
// than a placeholder.
func composeJobsByFile(cfg domain.RunConfig, files []string) map[string]string {
	jobs := make(map[string]string, len(files))
	for _, file := range files {
		if job := rules.ComposeJobName(rules.ComposeJobNameParams{Config: cfg, File: file}); job != "" {
			jobs[file] = job
		}
	}
	return jobs
}

type resolveEnvPortLinksParams struct {
	Interactive bool
	// LinkEnv is --link-env already given on the command line, or the wizard's
	// own answer: the links are authorized, so nothing is asked.
	LinkEnv bool
	// Declined is the wizard's refusal, which outranks everything: the question
	// was put and answered no.
	Declined   bool
	ProjectDir string
	EnvFiles   []domain.EnvFile
	Config     domain.RunConfig
}

// resolveEnvPortLinks offers the .env keys whose value holds one of the ports
// this run just settled. Nothing is inferred: a link is written from --link-env
// or from an explicit confirmation, never because the detection found a match.
func resolveEnvPortLinks(params resolveEnvPortLinksParams) []domain.EnvPortLink {
	candidates := detect.EnvPortCandidates(detect.EnvPortCandidatesParams{
		ProjectDir: params.ProjectDir,
		Files:      params.EnvFiles,
		Bases:      rules.EnvPortBases(params.Config),
		Existing:   params.Config.EnvPorts,
		JobsByDir:  rules.JobsByCwd(params.Config),
	})
	if len(candidates) == 0 || params.Declined {
		return nil
	}
	if params.LinkEnv {
		return candidates
	}
	if !params.Interactive {
		return nil
	}

	// The candidates go in the prompt's own description rather than a block
	// printed before it: the prompt renders on stderr inside its own frame, and a
	// command frames its stdout exactly once.
	confirmed, err := components.RunStandaloneConfirm(components.NewConfirm(components.NewConfirmParams{
		Title: domain.EnvPortLinkConfirm,
		Description: strings.Join(append(
			[]string{domain.EnvPortLinkDescription, ""},
			rules.EnvPortLinkLines(candidates, rules.EnvPortBases(params.Config))...), "\n"),
		DefaultYes: true,
	}))
	if err != nil || !confirmed {
		return nil
	}
	return candidates
}
