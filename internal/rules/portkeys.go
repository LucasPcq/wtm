package rules

import (
	"fmt"
	"path"
	"slices"
	"sort"

	"github.com/LucasPcq/wtm/internal/domain"
)

type PortKeyWritesParams struct {
	Config domain.RunConfig
	// Ports narrows the writes to the declarations routed to their .env. Nil
	// means every declared port, which is what a flag with no wizard behind it
	// asks for.
	Ports      []domain.PortRef
	ScansByDir map[string]domain.EnvPortScan
	EnvFiles   []domain.EnvFile
}

// PortKeyWrites names the declared ports no .env carries yet. The rewrite
// direction reports such a key and writes nothing, which is right for a rewrite;
// a project holding no key at all is the one that has not acquired the habit,
// and the key is what makes a hand-launched app read the worktree's port.
func PortKeyWrites(params PortKeyWritesParams) []domain.PortKeyWrite {
	byDir := make(map[string]domain.EnvFile, len(params.EnvFiles))
	for _, file := range params.EnvFiles {
		byDir[ScriptJobCwd(path.Dir(file.Target))] = file
	}

	var writes []domain.PortKeyWrite
	for _, job := range params.Config.Jobs {
		if len(job.Ports) == 0 || job.Kind == domain.JobKindTask || runsCompose(job) {
			continue
		}

		dir := ScriptJobCwd(job.Cwd)
		target, declared := byDir[dir]
		if !declared {
			target = conventionalEnvFile(dir)
		}

		for _, name := range sortedPortNames(job.Ports) {
			if !routedToEnv(params.Ports, domain.PortRef{Job: job.Name, Name: name}) {
				continue
			}
			// A file the project does not provision is written even when it
			// already holds the key: without the [env] target the worktree gets
			// no such file at all, so the value in the main checkout proves
			// nothing about the one that matters.
			if declared && carriedByValueFile(params.ScansByDir[dir], name, job.Ports[name]) {
				continue
			}
			writes = append(writes, domain.PortKeyWrite{
				Job:       job.Name,
				Port:      name,
				Base:      job.Ports[name],
				File:      target.Target,
				Template:  target.Template,
				AddTarget: !declared,
			})
		}
	}
	return writes
}

func routedToEnv(ports []domain.PortRef, ref domain.PortRef) bool {
	return ports == nil || slices.Contains(ports, ref)
}

// carriedByValueFile says the port was read from a file the app itself loads.
// A value found only in the committed template does not count: provisioning
// copies a parent's or main's .env under every strategy but `example`, so that
// key would reach no worktree.
func carriedByValueFile(scan domain.EnvPortScan, name string, base int) bool {
	if scan.Ports[name] != base {
		return false
	}
	_, _, isTemplate := TemplateTarget(path.Base(scan.SourceByVar[name]))
	return !isTemplate
}

// conventionalEnvFile is where a directory's values would live if the project
// provisioned any — the shape every other target in this repository has.
func conventionalEnvFile(dir string) domain.EnvFile {
	target := path.Join(dir, domain.EnvFileName)
	return domain.EnvFile{Target: target, Template: target + domain.EnvTemplateSuffixExample}
}

type PortKeyLinksParams struct {
	Writes   []domain.PortKeyWrite
	Existing []domain.EnvPortLink
}

// PortKeyLinks is the [[env_port]] entry each write implies: the key follows the
// port of the job whose directory it was written into. A link the config already
// carries is not repeated — the same key can be reached by this pass and by the
// detection that reads the project's env files.
func PortKeyLinks(params PortKeyLinksParams) []domain.EnvPortLink {
	declared := make(map[domain.EnvPortLink]bool, len(params.Existing))
	for _, link := range params.Existing {
		declared[link] = true
	}

	links := make([]domain.EnvPortLink, 0, len(params.Writes))
	for _, write := range params.Writes {
		link := domain.EnvPortLink{File: write.File, Key: write.Port, Job: write.Job, Port: write.Port}
		if declared[link] {
			continue
		}
		declared[link] = true
		links = append(links, link)
	}
	return links
}

type PortKeyTargetsParams struct {
	Writes   []domain.PortKeyWrite
	Existing []domain.EnvFile
}

// PortKeyTargets are the [env] targets the writes need and the project does not
// declare yet. Adding one is additive — no existing value changes, wtm simply
// provisions one more file.
func PortKeyTargets(params PortKeyTargetsParams) []domain.EnvFile {
	declared := make(map[string]bool, len(params.Existing))
	for _, file := range params.Existing {
		declared[file.Target] = true
	}

	var targets []domain.EnvFile
	for _, write := range params.Writes {
		if !write.AddTarget || declared[write.File] {
			continue
		}
		declared[write.File] = true
		targets = append(targets, domain.EnvFile{Target: write.File, Template: write.Template})
	}
	return targets
}

// PortRouteEnvPorts names the declarations the wizard routed to their own .env.
// A run that never put the question answers nil, which every caller reads as
// "every declared port" — the flag's meaning.
func PortRouteEnvPorts(answers domain.InitProjectAnswers) []domain.PortRef {
	if !answers.PortRoutesAsked {
		return nil
	}

	refs := make([]domain.PortRef, 0, len(answers.PortRoutes))
	for ref, route := range answers.PortRoutes {
		if route == domain.PortRouteEnv {
			refs = append(refs, ref)
		}
	}
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].Job != refs[j].Job {
			return refs[i].Job < refs[j].Job
		}
		return refs[i].Name < refs[j].Name
	})
	return refs
}

type JobsOnEnvRouteParams struct {
	Config domain.RunConfig
	Routes map[domain.PortRef]domain.PortRoute
}

// JobsOnEnvRoute names the jobs whose every declared port is routed to their
// .env. A job with one port left on its command still needs that command
// amended, so it is not exempt from the step that offers it.
func JobsOnEnvRoute(params JobsOnEnvRouteParams) []string {
	var jobs []string
	for _, job := range params.Config.Jobs {
		if len(job.Ports) == 0 {
			continue
		}
		onEnv := true
		for name := range job.Ports {
			if params.Routes[domain.PortRef{Job: job.Name, Name: name}] != domain.PortRouteEnv {
				onEnv = false
				break
			}
		}
		if onEnv {
			jobs = append(jobs, job.Name)
		}
	}
	sort.Strings(jobs)
	return jobs
}

type PortRouteRowsParams struct {
	Config domain.RunConfig
	// ComposeJobs are exempt: a stack reads its ports from the file wtm
	// templated, so neither route applies to it.
	ComposeJobs []string
	ScansByDir  map[string]domain.EnvPortScan
	EnvFiles    []domain.EnvFile
}

// PortRouteRows lists every service declaring a port, with the route it is on
// today: its command when that command already names the variable, its .env
// otherwise. The list is complete rather than narrowed to what is unresolved —
// a re-init shows what each job settled on, and lets it be changed.
func PortRouteRows(params PortRouteRowsParams) []domain.PortRouteRow {
	exempt := make(map[string]bool, len(params.ComposeJobs))
	for _, job := range params.ComposeJobs {
		exempt[job] = true
	}

	byDir := make(map[string]domain.EnvFile, len(params.EnvFiles))
	for _, file := range params.EnvFiles {
		byDir[ScriptJobCwd(path.Dir(file.Target))] = file
	}

	var rows []domain.PortRouteRow
	for _, job := range params.Config.Jobs {
		if exempt[job.Name] || len(job.Ports) == 0 || job.Kind == domain.JobKindTask || runsCompose(job) {
			continue
		}

		dir := ScriptJobCwd(job.Cwd)
		target, declared := byDir[dir]
		if !declared {
			target = conventionalEnvFile(dir)
		}

		for _, name := range sortedPortNames(job.Ports) {
			rows = append(rows, domain.PortRouteRow{
				Job:       job.Name,
				Port:      name,
				Base:      job.Ports[name],
				File:      target.Target,
				AddTarget: !declared,
				Route:     routeOf(routeOfParams{Job: job, Port: name, Scan: params.ScansByDir[dir]}),
			})
		}
	}
	return rows
}

type routeOfParams struct {
	Job  domain.JobConfig
	Port string
	Scan domain.EnvPortScan
}

// routeOf reads the route the project is already on. A .env carrying the value
// proves the job reads it — that is where the value was detected from — and a
// command naming the variable proves the other. Neither means the .env route,
// the one that also holds when its reader launches the app themselves.
func routeOf(params routeOfParams) domain.PortRoute {
	if params.Scan.Ports[params.Port] == params.Job.Ports[params.Port] {
		return domain.PortRouteEnv
	}
	if cmdMentions(params.Job.Cmd, params.Port) {
		return domain.PortRouteCommand
	}
	return domain.PortRouteEnv
}

type PortKeysReportedParams struct {
	// Applied are the writes that changed a file.
	Applied []domain.PortKeyWrite
	// Writes is the whole pass, and Targets the [env] entries it added: a file
	// that already held the right value still made the project provision it,
	// which is a change the reader has to be told about.
	Writes  []domain.PortKeyWrite
	Targets []domain.EnvFile
}

// PortKeysReported is what the run actually did, in declaration order.
func PortKeysReported(params PortKeysReportedParams) []domain.PortKeyWrite {
	added := make(map[string]bool, len(params.Targets))
	for _, target := range params.Targets {
		added[target.Target] = true
	}
	applied := make(map[domain.PortRef]bool, len(params.Applied))
	for _, write := range params.Applied {
		applied[domain.PortRef{Job: write.Job, Name: write.Port}] = true
	}

	var reported []domain.PortKeyWrite
	for _, write := range params.Writes {
		if applied[domain.PortRef{Job: write.Job, Name: write.Port}] || added[write.File] {
			reported = append(reported, write)
		}
	}
	return reported
}

func PortRouteWidths(rows []domain.PortRouteRow) (job, port int) {
	for _, row := range rows {
		job = max(job, len([]rune(row.Job)))
		port = max(port, len([]rune(portAssignment(row))))
	}
	return job, port
}

func PortRouteRowLabel(row domain.PortRouteRow, jobWidth, portWidth int) string {
	return fmt.Sprintf(domain.RouteListEntryFmt, pad(row.Job, jobWidth), pad(portAssignment(row), portWidth), routeLabel(row))
}

func portAssignment(row domain.PortRouteRow) string {
	return fmt.Sprintf(domain.RouteListPortFmt, row.Port, row.Base)
}

func routeLabel(row domain.PortRouteRow) string {
	if row.Route != domain.PortRouteEnv {
		return domain.RouteListCommand
	}
	label := fmt.Sprintf(domain.RouteListEnvFmt, row.File)
	if row.AddTarget {
		label += domain.PortKeyTargetSuffix
	}
	return label
}

func PortEntryWidths(entries []domain.PortEntry) (job, name int) {
	for _, entry := range entries {
		job = max(job, len([]rune(entry.Job)))
		name = max(name, len([]rune(entry.Name)))
	}
	return job, name
}

// PortEntryLabel is one line of the port list. A service nothing was detected
// for is spelled out rather than rendered as port zero: it is the one row on
// that step asking for something.
func PortEntryLabel(entry domain.PortEntry, jobWidth, nameWidth int) string {
	job, name := pad(entry.Job, jobWidth), pad(entry.Name, nameWidth)
	if entry.BindsNone {
		return fmt.Sprintf(domain.PortListBindsNoneFmt, job) + domain.PortKeyColumnSep + domain.PortListBindsNone
	}
	if entry.Base <= 0 {
		return fmt.Sprintf(domain.PortListUndeclaredFmt, job, name) + domain.PortKeyColumnSep + domain.PortListUndeclared
	}
	return fmt.Sprintf(domain.PortListEntryFmt, job, name, entry.Base)
}

func PortEntryEditLabel(entry domain.PortEntry, input string, jobWidth, nameWidth int) string {
	return fmt.Sprintf(domain.PortListEditFmt, pad(entry.Job, jobWidth), pad(entry.Name, nameWidth), input)
}
