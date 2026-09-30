// Package initrun is `wtm run init`: detect the services, ask about them, then
// write run.toml, the compose files and the .env files the answers call for.
package initrun

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/run/addressing"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/compose"
	"github.com/LucasPcq/wtm/internal/service/detect"
	envsvc "github.com/LucasPcq/wtm/internal/service/env"
	"github.com/LucasPcq/wtm/internal/service/proxy"
	"github.com/LucasPcq/wtm/internal/service/runconfig"
)

type Request struct {
	PatchCompose  bool
	LinkEnv       bool
	WritePortKeys bool
}

type Presenter interface {
	flow.Presenter
	Initialized(Outcome) error
}

// Wizard is the services questionnaire. It stays a seam of its own rather than
// a flow.Session: most of its screens edit structured rows (ports, runners,
// scopes, namespaces, routes, commands, profiles) that no flow.StepKind renders,
// and adding a kind means teaching every surface to draw it.
type Wizard interface {
	AskServices(Question) (domain.InitProjectAnswers, error)
}

// Question is everything the wizard reads. Prefill is nil on a first init, and
// on a re-init names what run.toml already declares so the wizard opens on it.
type Question struct {
	ProjectDir   string
	Detection    domain.InitDetectionResult
	Existing     domain.RunConfig
	Prefill      *Prefill
	PatchCompose bool
	EnvScans     map[string]domain.EnvPortScan
	EnvLines     map[string][]domain.EnvLine
	EnvFiles     []domain.EnvFile
}

type Prefill struct {
	DockerFiles   map[string]bool
	ScriptIndices map[int]bool
}

type Params struct {
	Context   flow.Context
	Request   Request
	Prompter  flow.Prompter
	Wizard    Wizard
	Presenter Presenter
}

type Outcome struct {
	Aborted         bool
	NothingDetected bool
	Report          Report
}

// Report is what the run did and what it left to the reader, computed once the
// files are written so every line reads the state on disk.
type Report struct {
	RunPath             string
	Added, Removed      int
	Kept                int
	Detected            rules.DetectedPortsOutcome
	JobsByFile          map[string]string
	SharingLines        []string
	NamePatches         map[string][]domain.ComposeAbsoluteName
	NamesWithheld       []domain.ComposeAbsoluteName
	Links               []domain.EnvPortLink
	LinkBases           map[domain.PortRef]int
	PortKeys            []domain.PortKeyWrite
	Unported            []string
	Ignoring            []domain.JobCmdFix
	CommandOnly         []string
	ProxyCollisionLines []string
	ProxyInstallLines   []string
	AddressingDrift     *flow.Notice
}

func Run(params Params) (Outcome, error) {
	ctx := params.Context
	envFiles := ctx.Config.Project.Env.Files

	var detection domain.InitDetectionResult
	var envScans map[string]domain.EnvPortScan
	_ = params.Presenter.Stage(flow.StageParams{
		Message: domain.RunInitDetectingMessage,
		Work: func() error {
			detection = detect.ProjectEnvironment(ctx.ProjectDir)
			detection.ComposeScans = compose.ScanAll(compose.ScanAllParams{
				ProjectDir: ctx.ProjectDir,
				Files:      detection.DockerComposeFiles,
				Project:    filepath.Base(ctx.ProjectDir),
			})
			envScans = detect.ScanEnvPorts(detect.ScanEnvPortsParams{
				ProjectDir: ctx.ProjectDir,
				Files:      detection.EnvFiles,
			})
			return nil
		},
	})

	existing, err := runconfig.Load(ctx.StateDir)
	if err != nil {
		return Outcome{}, fmt.Errorf("load run.toml: %w", err)
	}

	answers, err := askServices(askServicesParams{
		Params:    params,
		Detection: detection,
		Existing:  existing,
		EnvScans:  envScans,
		EnvLines: detect.EnvLines(detect.EnvPortCandidatesParams{
			ProjectDir: ctx.ProjectDir,
			Files:      envFiles,
		}),
	})
	// A user backing out changed nothing and says nothing: the wizard's own
	// screen was the last thing drawn.
	if errors.Is(err, domain.ErrUserAborted) {
		return Outcome{Aborted: true}, nil
	}
	if err != nil {
		return Outcome{}, err
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
		ProjectDir:  ctx.ProjectDir,
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
		result := Outcome{NothingDetected: true}
		return result, params.Presenter.Initialized(result)
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

	if answers.TouchesAsked {
		outcome.Config = rules.ApplyTouchChoices(rules.ApplyTouchChoicesParams{Config: outcome.Config, Choices: answers.Touches})
	}

	// After the profiles are settled: the step re-proposes them from the config
	// on disk, so a lifted job inserted any earlier is discarded — and a profile
	// that no longer starts the database leaves every worktree addressing one
	// that was never created.
	outcome.Config = rules.JoinSharedProfiles(rules.JoinSharedProfilesParams{
		Config: outcome.Config,
		Shared: answers.SharedServices,
	})

	links := resolveEnvPortLinks(resolveEnvPortLinksParams{
		Prompter: params.Prompter,
		// The wizard already put the question as a step; asking again outside it
		// is the orphaned prompt this flow used to end on.
		Ask:        !answers.EnvLinksAsked,
		LinkEnv:    params.Request.LinkEnv || (answers.EnvLinksAsked && answers.LinkEnv),
		Declined:   answers.EnvLinksAsked && !answers.LinkEnv,
		ProjectDir: ctx.ProjectDir,
		EnvFiles:   envFiles,
		Config:     outcome.Config,
	})
	outcome.Config.EnvPorts = append(outcome.Config.EnvPorts, links...)

	// Writing a committed template is never inferred, exactly as the compose
	// patching is not: it takes the flag, or the wizard's route answer.
	var portKeys []domain.PortKeyWrite
	if params.Request.WritePortKeys || answers.PortRoutesAsked {
		portKeys = rules.PortKeyWrites(rules.PortKeyWritesParams{
			Config:     outcome.Config,
			Ports:      rules.PortRouteEnvPorts(answers),
			ScansByDir: envScans,
			EnvFiles:   envFiles,
		})
	}
	writtenKeys, err := envsvc.WritePortKeys(envsvc.WritePortKeysParams{ProjectDir: ctx.ProjectDir, Writes: portKeys})
	if err != nil {
		return Outcome{}, err
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
		ProjectDir:  ctx.ProjectDir,
		ByFile:      outcome.Patches,
		NamesByFile: namePatches,
	}); err != nil {
		return Outcome{}, err
	}
	if err := runconfig.Save(runconfig.SaveParams{StateDir: ctx.StateDir, Config: outcome.Config}); err != nil {
		return Outcome{}, err
	}

	// Last of the writes: a provisioning target with no link behind it would
	// have wtm copying a file nothing in run.toml speaks about.
	addedTargets := rules.PortKeyTargets(rules.PortKeyTargetsParams{
		Writes:   portKeys,
		Existing: envFiles,
	})
	if len(addedTargets) > 0 {
		if err := envsvc.AddEnvTargets(envsvc.AddEnvTargetsParams{
			StateDir: ctx.StateDir,
			Project:  ctx.Config.Project,
			Targets:  addedTargets,
		}); err != nil {
			return Outcome{}, fmt.Errorf("add env targets: %w", err)
		}
	}

	result := Outcome{Report: report(reportParams{
		Context:     ctx,
		Detection:   detection,
		Answers:     answers,
		Outcome:     outcome,
		NamePatches: namePatches,
		NamePlan:    namePlan,
		Links:       links,
		PortKeys: rules.PortKeysReported(rules.PortKeysReportedParams{
			Applied: writtenKeys,
			Writes:  portKeys,
			Targets: addedTargets,
		}),
	})}
	return result, params.Presenter.Initialized(result)
}

type askServicesParams struct {
	Params    Params
	Detection domain.InitDetectionResult
	Existing  domain.RunConfig
	EnvScans  map[string]domain.EnvPortScan
	EnvLines  map[string][]domain.EnvLine
}

// askServices gathers the services selection either from the wizard or, when
// nobody can answer it, straight from detection. On a re-run the wizard is
// pre-filled with what run.toml already declares so the subsequent merge is
// additive rather than a fresh overwrite.
func askServices(params askServicesParams) (domain.InitProjectAnswers, error) {
	request := params.Params.Request
	if !params.Params.Prompter.Interactive() {
		return rules.AutoServicesAnswers(rules.AutoServicesAnswersParams{
			Detection:    params.Detection,
			PatchCompose: request.PatchCompose,
		}), nil
	}

	var prefill *Prefill
	if rules.IsRunInitialized(params.Existing) {
		prefill = &Prefill{
			DockerFiles:   rules.DockerFilesConfigured(params.Existing, params.Detection.DockerComposeFiles),
			ScriptIndices: rules.ScriptsConfigured(params.Existing, params.Detection.PackageScripts, params.Detection.PackageManager),
		}
	}

	ctx := params.Params.Context
	return params.Params.Wizard.AskServices(Question{
		ProjectDir:   ctx.ProjectDir,
		Detection:    params.Detection,
		Existing:     params.Existing,
		Prefill:      prefill,
		PatchCompose: request.PatchCompose,
		EnvScans:     params.EnvScans,
		EnvLines:     params.EnvLines,
		EnvFiles:     ctx.Config.Project.Env.Files,
	})
}

type resolveEnvPortLinksParams struct {
	Prompter flow.Prompter
	// Ask is false once the wizard put the question itself.
	Ask bool
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
	if !params.Ask || !params.Prompter.Interactive() {
		return nil
	}

	// The candidates go in the prompt's own description rather than a block
	// printed before it: the prompt renders on stderr inside its own frame, and a
	// command frames its stdout exactly once.
	confirmed, err := params.Prompter.Confirm(flow.ConfirmParams{
		Title: domain.EnvPortLinkConfirm,
		Description: strings.Join(append(
			[]string{domain.EnvPortLinkDescription, ""},
			rules.EnvPortLinkLines(candidates, rules.EnvPortBases(params.Config))...), "\n"),
		DefaultYes: true,
	})
	if err != nil || !confirmed {
		return nil
	}
	return candidates
}

type reportParams struct {
	Context     flow.Context
	Detection   domain.InitDetectionResult
	Answers     domain.InitProjectAnswers
	Outcome     rules.DetectedPortsOutcome
	NamePatches map[string][]domain.ComposeAbsoluteName
	NamePlan    rules.ComposeNamePlan
	Links       []domain.EnvPortLink
	PortKeys    []domain.PortKeyWrite
}

func report(params reportParams) Report {
	ctx, cfg := params.Context, params.Outcome.Config

	// Re-read after the writes: the reports below say what is still missing, and
	// the scan they were computed from predates the keys this run just wrote.
	envScans := detect.ScanEnvPorts(detect.ScanEnvPortsParams{
		ProjectDir: ctx.ProjectDir,
		Files:      params.Detection.EnvFiles,
	})
	composeJobs := rules.ComposeJobsFor(rules.ComposeJobsParams{Config: cfg, Files: params.Answers.DockerComposeFiles})
	proxyPort := rules.ProxyPort(ctx.Config.Global)

	result := Report{
		RunPath:    filepath.Join(ctx.StateDir, domain.RunFileName),
		Added:      len(params.Outcome.Merge.Added),
		Removed:    len(params.Outcome.Removed),
		Kept:       len(params.Outcome.Merge.Skipped),
		Detected:   params.Outcome,
		JobsByFile: composeJobsByFile(cfg, params.Answers.DockerComposeFiles),
		SharingLines: rules.ComposeSharingLines(rules.ComposeSharingLinesParams{
			Renamed:  params.Outcome.Renamed,
			Unlinked: params.Outcome.Unlinked,
		}),
		NamePatches:   params.NamePatches,
		NamesWithheld: params.NamePlan.Withheld,
		Links:         params.Links,
		LinkBases:     rules.EnvPortBases(cfg),
		PortKeys:      params.PortKeys,
		Unported:      rules.ServicesWithoutPorts(cfg),
		Ignoring: rules.JobsMissingPortRef(rules.JobsMissingPortRefParams{
			Config: cfg,
			Exempt: append(composeJobs,
				rules.JobsReadingTheirEnv(rules.JobsReadingTheirEnvParams{Config: cfg, ScansByDir: envScans})...),
		}),
		CommandOnly: rules.JobsIsolatedByCommand(rules.JobsIsolatedByCommandParams{
			Config:     cfg,
			Exempt:     composeJobs,
			ScansByDir: envScans,
		}),
		ProxyCollisionLines: rules.ProxyPortCollisionLines(rules.ProxyPortCollisions(rules.ProxyPortCollisionsParams{
			Config:    cfg,
			ProxyPort: proxyPort,
		}), proxyPort),
		ProxyInstallLines: rules.ProxyInstallHintLines(rules.ProxyInstallHintParams{
			Config:     cfg,
			Status:     proxy.NewRedirector(proxy.RedirectorParams{}).Inspect(),
			ExampleURL: fmt.Sprintf(domain.ProxyURLFmt, domain.ProxyHostShape, proxyPort),
		}),
	}
	// The main checkout is the one no command ever provisions, so it is the one
	// the addressing just chosen leaves behind.
	if notice, ok := addressing.Notice(addressing.Params{Context: ctx, WorkDirs: []string{ctx.ProjectDir}}); ok {
		result.AddressingDrift = &notice
	}
	return result
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
