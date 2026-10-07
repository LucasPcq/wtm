package run

import (
	"fmt"
	"github.com/LucasPcq/wtm/internal/service/proxy"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/run/runctx"
	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/run/initrun"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/tui/components"
	initwizard "github.com/LucasPcq/wtm/internal/tui/inittui"
)

// newInitCmd creates the wtm run init subcommand — the dedicated entry point
// that configures the run module, kept out of the global
// `wtm init` wizard so users who never touch `run` aren't bothered by it.
func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdInit,
		Short: "Configure the run module (services & tasks) for this repo",
		Long: "Set up run.toml by detecting docker-compose files and package.json scripts and turning\n" +
			"the selected ones into jobs.\n\n" +
			"In a TTY, opens a wizard to pick which ones to include; with --yes (or piped),\n" +
			"auto-generates from detection. Re-running pre-fills every step from the existing\n" +
			"run.toml: what stays checked is kept, what you uncheck is removed along with the\n" +
			"profile entries and .env links naming it. Only jobs this wizard proposed are ever\n" +
			"removed — one added with `wtm run job add` is never listed, so never touched.\n" +
			"An unattended run asks nothing and removes nothing.\n\n" +
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
			"with a consequence outside wtm: named URLs are served by the run proxy, so they\n" +
			"answer while `wtm run` runs the job and not when you start it yourself. A project\n" +
			"whose author launches their own dev servers wants ports.\n\n" +
			"Every service that declares the port it listens on is then offered a name of its\n" +
			"own — <job>.<worktree>.<repo>.localhost, served by the proxy — so two worktrees\n" +
			"stop sharing a cookie jar. A port a job only dials (DB_PORT, REDIS_PORT) is never\n" +
			"offered: a name nothing answers under is worse than no name at all.",
		Example: `  # The wizard
  wtm run init

  # Unattended, from detection
  wtm run init --yes

  # Also rewrite compose host ports and link the .env port keys
  wtm run init --yes --patch-compose --link-env`,
		Args: cobra.NoArgs,
		RunE: runRunInit,
	}
	shared.AddYesFlag(cmd, "Run unattended: auto-generate from detection; never prompt")
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

	yes, _ := cmd.Flags().GetBool(domain.FlagYes)
	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	patchCompose, _ := cmd.Flags().GetBool(domain.FlagPatchCompose)
	linkEnv, _ := cmd.Flags().GetBool(domain.FlagLinkEnv)
	writePortKeys, _ := cmd.Flags().GetBool(domain.FlagWritePortKeys)
	interactive := shared.Interactive(shared.UnattendedParams{TTY: runctx.IsTTY(), Format: format, Yes: yes})

	outcome, err := initrun.Run(initrun.Params{
		Context:   shared.FlowContext(res),
		Request:   initrun.Request{PatchCompose: patchCompose, LinkEnv: linkEnv, WritePortKeys: writePortKeys, Redirection: inspectRedirection()},
		Prompter:  shared.FlowPrompter(shared.FlowPrompterParams{Interactive: interactive}),
		Wizard:    servicesWizard{},
		Presenter: initPresenter{CLIPresenter: shared.NewPresenter(cmd, format), animate: shared.Animate(cmd, interactive)},
	})
	if err != nil {
		return err
	}
	if outcome.Aborted {
		return shared.EndAborted(cmd)
	}
	return nil
}

// inspectRedirection is a seam: what a run reports about the redirection
// depends on the platform, and a test pins it.
var inspectRedirection = func() domain.ProxyStatus {
	return proxy.NewRedirector(proxy.RedirectorParams{}).Inspect()
}

type servicesWizard struct{}

func (servicesWizard) AskServices(question initrun.Question) (domain.InitProjectAnswers, error) {
	var prefill *initwizard.SectionPrefill
	if question.Prefill != nil {
		prefill = &initwizard.SectionPrefill{
			DockerFiles:   question.Prefill.DockerFiles,
			ScriptIndices: question.Prefill.ScriptIndices,
		}
	}
	return initwizard.RunServicesWizard(initwizard.ServicesWizardParams{
		ProjectDir:   question.ProjectDir,
		Detection:    question.Detection,
		Existing:     question.Existing,
		Prefill:      prefill,
		PatchCompose: question.PatchCompose,
		EnvScans:     question.EnvScans,
		EnvLines:     question.EnvLines,
		EnvFiles:     question.EnvFiles,
	})
}

// initPresenter animates detection only where a wizard follows: `--yes` on a
// terminal has always run its detection without a spinner.
type initPresenter struct {
	shared.CLIPresenter
	animate bool
}

func (p initPresenter) Stage(params flow.StageParams) error {
	return components.RunLoading(components.LoadingParams{
		Message: params.Message,
		Animate: p.animate,
		Work:    params.Work,
	})
}

func (p initPresenter) Initialized(outcome initrun.Outcome) error {
	w := p.Cmd.OutOrStdout()
	if outcome.NothingDetected {
		output.Frame(w, func(w io.Writer) {
			output.Unchanged(w, domain.RunInitNothingDetected)
			output.Blank(w)
			output.NextStep(w, output.NextStepParams{Command: domain.RunInitByHandJob, Note: domain.RunInitByHandJobNote})
			output.NextStep(w, output.NextStepParams{Command: domain.RunInitByHandProfile, Note: domain.RunInitByHandProfNote})
		})
		return nil
	}

	report := outcome.Report
	output.Frame(w, func(w io.Writer) {
		// The jobs are counted, not named: the reader ticked them one by one in the
		// wizard, and run.toml is where they live now.
		output.Success(w, fmt.Sprintf(domain.RunInitConfiguredFmt, report.RunPath, rules.Tally(
			domain.TallyPart{Count: report.Added, Label: domain.TallyAdded},
			domain.TallyPart{Count: report.Removed, Label: domain.TallyRemoved},
			domain.TallyPart{Count: report.Kept, Label: domain.TallyKept},
		)))
		detected := report.Detected
		output.DetectedPortsReport(w, output.DetectedPortsReportParams{
			Patched:       detected.Patches,
			Written:       detected.Written,
			Withheld:      detected.Withheld,
			JobsByFile:    report.JobsByFile,
			Dropped:       detected.Dropped,
			Unreadable:    detected.Unreadable,
			Changed:       detected.Changed,
			Orphaned:      detected.Orphaned,
			EnvWritten:    detected.EnvWritten,
			EnvSources:    detected.EnvSources,
			EnvUnreadable: detected.EnvUnreadable,
		})
		if len(report.SharingLines) > 0 {
			output.Blank(w)
			output.Callout(w, domain.ComposeSharingTitle, report.SharingLines)
		}
		output.ComposeNamesReport(w, output.ComposeNamesReportParams{
			Patched:  report.NamePatches,
			Withheld: report.NamesWithheld,
		})
		output.EnvPortLinksReport(w, report.Links, report.LinkBases)
		output.PortKeysReport(w, report.PortKeys)
		// Last, and alone in a frame: everything above is what the run did, this
		// is what it could not do without the reader.
		output.PortIsolationReport(w, output.PortIsolationReportParams{
			Unported: report.Unported,
			Ignoring: report.Ignoring,
		})
		output.PortCommandOnlyReport(w, report.CommandOnly)
		if len(report.ProxyCollisionLines) > 0 {
			output.Callout(w, domain.ProxyPortCollisionTitle, report.ProxyCollisionLines)
		}
		if len(report.ProxyInstallLines) > 0 {
			output.Callout(w, domain.ProxyInstallHintTitle, report.ProxyInstallLines)
		}
		if drift := report.AddressingDrift; drift != nil {
			output.Callout(w, drift.Text, drift.Lines)
		}
		output.Blank(w)
		output.NextSteps(w, []output.NextStepParams{
			{Command: domain.RunInitNextUp, Note: domain.RunInitNextUpNote},
			{Command: domain.RunInitNextJobAdd, Note: domain.RunInitNextJobAddNote},
		})
	})
	return nil
}
