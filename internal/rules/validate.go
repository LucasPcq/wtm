package rules

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/LucasPcq/wtm/internal/domain"
)

// Validate checks that all config values are within their allowed sets.
func Validate(cfg domain.Config) error {
	if err := ValidateEnvStrategy(cfg.Project.Env.Strategy); err != nil {
		return err
	}
	if err := ValidateEnvFiles(cfg.Project.Env.Files); err != nil {
		return err
	}
	if err := ValidateShellType(cfg.Global.Shell); err != nil {
		return err
	}
	return nil
}

// ValidateEnvFiles rejects entries with an empty target, duplicate targets, or a
// template that is not a recognized template of its target.
func ValidateEnvFiles(files []domain.EnvFile) error {
	seen := make(map[string]bool, len(files))
	for _, f := range files {
		if f.Target == "" {
			return domain.ErrEnvFileNoTarget
		}
		if seen[f.Target] {
			return fmt.Errorf("%w: %s", domain.ErrEnvFileDuplicateTarget, f.Target)
		}
		seen[f.Target] = true

		if f.Template != "" && !isTemplateOf(templateMatchParams{Target: f.Target, Template: f.Template}) {
			return fmt.Errorf("%w: %s is not a template of %s", domain.ErrEnvFileBadTemplate, f.Template, f.Target)
		}
	}
	return nil
}

// SudoDeletePathParams holds the inputs for ValidateSudoDeletePath.
type SudoDeletePathParams struct {
	// Path is the directory that would be removed with `sudo rm -rf`.
	Path string
	// HomeDir is the current user's home directory ("" when it cannot be resolved).
	HomeDir string
	// ProjectDir is the repository root the worktree belongs to.
	ProjectDir string
}

// ValidateSudoDeletePath rejects a privileged recursive delete of an obviously
// dangerous path: a non-absolute path, a filesystem root, the home directory, the
// repository root, or an ancestor of it. Purely lexical — no filesystem access.
func ValidateSudoDeletePath(params SudoDeletePathParams) error {
	if !filepath.IsAbs(params.Path) {
		return domain.ErrUnsafeSudoDeletePath
	}

	path := filepath.Clean(params.Path)
	if path == filepath.Dir(path) {
		return domain.ErrUnsafeSudoDeletePath
	}
	if params.HomeDir != "" && path == filepath.Clean(params.HomeDir) {
		return domain.ErrUnsafeSudoDeletePath
	}
	if params.ProjectDir != "" {
		project := filepath.Clean(params.ProjectDir)
		if path == project || isAncestor(path, project) {
			return domain.ErrUnsafeSudoDeletePath
		}
	}
	return nil
}

// isAncestor reports whether ancestor is a strict parent directory of child. Both
// must be cleaned absolute paths. Segment-aware so "/a/b" is not treated as an
// ancestor of "/a/bc".
func isAncestor(ancestor, child string) bool {
	rel, err := filepath.Rel(ancestor, child)
	if err != nil {
		return false
	}
	return rel != "." && !strings.HasPrefix(rel, "..")
}

// templateMatchParams holds the inputs for isTemplateOf.
type templateMatchParams struct {
	Target   string
	Template string
}

func isTemplateOf(p templateMatchParams) bool {
	for _, c := range TemplateCandidates(p.Target) {
		if c == p.Template {
			return true
		}
	}
	return false
}

// ValidateEnvStrategy returns ErrInvalidEnvStrategy if s is not a known value.
func ValidateEnvStrategy(s domain.EnvStrategy) error {
	switch s {
	case domain.EnvStrategyExample, domain.EnvStrategyMain, domain.EnvStrategyParent:
		return nil
	default:
		return domain.ErrInvalidEnvStrategy
	}
}

// ValidateShellType returns ErrInvalidShellType if s is not a known value.
func ValidateShellType(s domain.ShellType) error {
	switch s {
	case domain.ShellZsh, domain.ShellBash, domain.ShellFish:
		return nil
	default:
		return domain.ErrInvalidShellType
	}
}

// ValidateRelocateTarget checks the --to value for `relocate`. An empty
// string means the flag was not provided (relocate then uses the current
// base_path) and is allowed. A non-empty value must be a repo-relative path:
// whitespace-only and absolute paths are rejected.
func ValidateRelocateTarget(to string) error {
	if to == "" {
		return nil
	}
	if strings.TrimSpace(to) == "" || filepath.IsAbs(to) {
		return domain.ErrInvalidBasePath
	}
	return nil
}

// ValidateRun checks for structural errors in the run config and returns
// warnings for ambiguous-but-not-fatal cases. A non-empty error slice means
// the config should be rejected.
func ValidateRun(cfg domain.RunConfig) (warnings []string, errs []string) {
	jobNames := map[string]bool{}
	for _, j := range cfg.Jobs {
		if j.Name == "" {
			errs = append(errs, "job with empty name")
			continue
		}
		if strings.ContainsFunc(j.Name, unicode.IsSpace) {
			errs = append(errs, fmt.Sprintf(domain.RunJobNameSpacesFmt, j.Name))
		}
		if jobNames[j.Name] {
			errs = append(errs, fmt.Sprintf("duplicate job name %q — names must be unique across the file", j.Name))
		}
		jobNames[j.Name] = true

		if j.Cmd == "" {
			errs = append(errs, fmt.Sprintf("job %q: cmd is required", j.Name))
		}

		switch j.Kind {
		case domain.JobKindService:
		case domain.JobKindTask:
			if j.Stop != "" {
				errs = append(errs, fmt.Sprintf("job %q: tasks cannot declare a stop command", j.Name))
			}
		case "":
			errs = append(errs, fmt.Sprintf("job %q: kind is required (service or task)", j.Name))
		default:
			errs = append(errs, fmt.Sprintf("job %q: unknown kind %q (expected service or task)", j.Name, j.Kind))
		}
	}

	errs = append(errs, validateJobRelations(cfg, jobNames)...)
	errs = append(errs, ValidateRunPorts(cfg)...)
	errs = append(errs, ValidateAddressing(cfg)...)
	errs = append(errs, ValidateConcurrency(cfg)...)

	seenProfiles := map[string]bool{}
	defaultCount := 0
	for _, p := range cfg.Profiles {
		if p.Name == "" {
			errs = append(errs, "profile with empty name")
			continue
		}
		if strings.ContainsFunc(p.Name, unicode.IsSpace) {
			errs = append(errs, fmt.Sprintf(domain.RunProfileNameSpacesFmt, p.Name))
		}
		if seenProfiles[p.Name] {
			errs = append(errs, fmt.Sprintf("duplicate profile name %q — names must be unique across the file", p.Name))
		}
		seenProfiles[p.Name] = true

		for _, ref := range p.Jobs {
			if !jobNames[ref] {
				errs = append(errs, fmt.Sprintf("profile %q references unknown job %q", p.Name, ref))
			}
		}

		if p.Default {
			defaultCount++
		}
	}
	if defaultCount > 1 {
		errs = append(errs, fmt.Sprintf("%d profiles marked as default — only one profile can be the default", defaultCount))
	}

	return warnings, errs
}

// validateJobRelations checks what a job says about the other jobs and about
// its own ports: a runner names jobs that exist and is not one of them, and a
// service either declares ports or says it binds none — never both.
func validateJobRelations(cfg domain.RunConfig, jobNames map[string]bool) []string {
	var errs []string
	for _, job := range cfg.Jobs {
		if job.BindsNoPort && len(job.Ports) > 0 {
			errs = append(errs, fmt.Sprintf("job %q: binds_no_port contradicts the %d port(s) it declares", job.Name, len(job.Ports)))
		}
		if job.BindsNoPort && job.Kind == domain.JobKindTask {
			errs = append(errs, fmt.Sprintf("job %q: binds_no_port says nothing about a task, which binds nothing by nature", job.Name))
		}

		seen := map[string]bool{}
		for _, ref := range job.Runs {
			if ref == job.Name {
				errs = append(errs, fmt.Sprintf("job %q: runs itself", job.Name))
				continue
			}
			if !jobNames[ref] {
				errs = append(errs, fmt.Sprintf("job %q: runs unknown job %q", job.Name, ref))
				continue
			}
			if seen[ref] {
				errs = append(errs, fmt.Sprintf("job %q: runs %q twice", job.Name, ref))
			}
			seen[ref] = true
		}
	}
	errs = append(errs, runnerCycles(cfg)...)
	return errs
}

// runnerCycles refuses a runner reachable from itself. Two jobs each declaring
// they run the other would make every rule that walks the relation — the ports
// a runner inherits, the jobs it locks out — recurse forever.
func runnerCycles(cfg domain.RunConfig) []string {
	runs := make(map[string][]string, len(cfg.Jobs))
	for _, job := range cfg.Jobs {
		runs[job.Name] = job.Runs
	}

	var errs []string
	for _, job := range cfg.Jobs {
		if len(job.Runs) == 0 {
			continue
		}
		seen := map[string]bool{job.Name: true}
		var queue []string
		for _, ref := range job.Runs {
			if ref != job.Name {
				queue = append(queue, runs[ref]...)
			}
		}
		for len(queue) > 0 {
			ref := queue[0]
			queue = queue[1:]
			if ref == job.Name {
				errs = append(errs, fmt.Sprintf("job %q: runs itself through %s", job.Name, strings.Join(job.Runs, ", ")))
				break
			}
			if seen[ref] {
				continue
			}
			seen[ref] = true
			queue = append(queue, runs[ref]...)
		}
	}
	return errs
}

// ValidateRunPorts checks what only run.toml can answer for: whether the
// declared ports can be exported at all, and whether they can coexist. Unlike
// the structural checks around it, this one is enforced when the file is read
// rather than only when it is written — a port layout that cannot work produces
// an EADDRINUSE with nothing pointing back at run.toml, and this is the last
// moment the problem is still explainable.
func ValidateRunPorts(cfg domain.RunConfig) []string {
	var errs []string
	for _, job := range cfg.Jobs {
		for _, name := range sortedPortNames(job.Ports) {
			if !IsEnvVarName(name) {
				errs = append(errs, fmt.Sprintf("job %q: port %q is not a valid environment variable name", job.Name, name))
			}
			if base := job.Ports[name]; base < domain.PortMin || base > domain.PortMax {
				errs = append(errs, fmt.Sprintf("job %q: port %s is %d, outside %d-%d", job.Name, name, base, domain.PortMin, domain.PortMax))
			}
		}
	}

	if cfg.PortOffsetBlock < 0 {
		errs = append(errs, fmt.Sprintf("port_offset_block is %d — it must be positive (omit it for the default of %d)", cfg.PortOffsetBlock, domain.PortOffsetBlock))
	}

	block := EffectivePortOffsetBlock(cfg)
	for _, c := range PortCollisions(cfg) {
		if c.Worktrees == 0 {
			errs = append(errs, fmt.Sprintf(
				"ports %s (job %q) and %s (job %q) both declare base %d — they would bind the same port in every worktree",
				c.A.Name, c.A.Job, c.B.Name, c.B.Job, c.A.Base))
			continue
		}
		errs = append(errs, fmt.Sprintf(
			"ports %s (job %q, base %d) and %s (job %q, base %d) collide %d worktree(s) apart with a port_offset_block of %d — move one base so the gap between them is not a multiple of %d",
			c.A.Name, c.A.Job, c.A.Base, c.B.Name, c.B.Job, c.B.Base, c.Worktrees, block, block))
	}

	errs = append(errs, validateJobURLs(cfg)...)
	errs = append(errs, validateEnvPortLinks(cfg)...)
	return append(errs, validateEnvValueLinks(cfg)...)
}

// validateJobURLs checks what run.toml can answer for on its own. The collision
// case is the one that matters: without it the proxy arbitrates silently and
// half the traffic goes to the wrong worktree.
func validateJobURLs(cfg domain.RunConfig) []string {
	var errs []string
	claimed := map[string]string{}

	for _, job := range cfg.Jobs {
		if job.URL == nil {
			continue
		}
		if _, declared := job.Ports[job.URL.Port]; !declared {
			errs = append(errs, fmt.Sprintf("job %q: url.port names %s, which the job does not declare", job.Name, job.URL.Port))
		}
		if job.URL.Host != "" && !IsHostLabels(job.URL.Host) {
			errs = append(errs, fmt.Sprintf("job %q: url.host %q is not a valid hostname — lowercase letters, digits and dashes, dot-separated", job.Name, job.URL.Host))
			continue
		}

		host := JobHostLabel(job)
		if owner, taken := claimed[host]; taken {
			errs = append(errs, fmt.Sprintf("jobs %q and %q both publish host %q — give one an explicit url.host", owner, job.Name, host))
			continue
		}
		claimed[host] = job.Name
	}
	return errs
}

// JobHostLabel is the host segment a job publishes under: what it declared, else
// its own name made safe for DNS.
func JobHostLabel(job domain.JobConfig) string {
	if job.URL == nil {
		return ""
	}
	if job.URL.Host != "" {
		return job.URL.Host
	}
	return HostLabel(job.Name)
}

// validateEnvPortLinks checks what run.toml can answer for on its own: that each
// link names a port the file declares, a key a shell could export, and a pair no
// other link already claims. Whether the file it names is a configured env target
// needs .wtm.toml and is checked by ValidateEnvPortTargets instead.
func validateEnvPortLinks(cfg domain.RunConfig) []string {
	bases := EnvPortBases(cfg)

	var errs []string
	seen := map[domain.EnvPortLink]bool{}
	for _, link := range cfg.EnvPorts {
		if link.File == "" {
			errs = append(errs, fmt.Sprintf("env_port %s: file is required", link.Key))
		}
		if !IsEnvVarName(link.Key) {
			errs = append(errs, fmt.Sprintf("env_port in %s: %q is not a valid environment variable name", link.File, link.Key))
		}
		if _, declared := EnvPortBaseFor(bases, link); !declared {
			errs = append(errs, envPortLinkError(bases, link))
		}

		// A key may follow several ports — an origin list names one per
		// front-end — but never the same one twice: that is a line written by
		// hand two times, and the second moves nothing the first did not.
		pair := domain.EnvPortLink{File: link.File, Key: link.Key, Job: link.Job, Port: link.Port}
		if seen[pair] {
			errs = append(errs, fmt.Sprintf(domain.EnvPortLinkTwiceFmt, link.Key, link.File, link.Port))
		}
		seen[pair] = true
	}
	return errs
}

// validateEnvValueLinks checks what run.toml can answer for on its own about the
// keys wtm writes in full: a job that exists, a template whose every placeholder
// resolves, and a key no other table already writes.
func validateEnvValueLinks(cfg domain.RunConfig) []string {
	if len(cfg.EnvValues) == 0 {
		return nil
	}

	byName := make(map[string]domain.JobConfig, len(cfg.Jobs))
	for _, job := range cfg.Jobs {
		byName[job.Name] = job
	}
	ports := map[domain.EnvKeyRef]bool{}
	for _, link := range cfg.EnvPorts {
		ports[domain.EnvKeyRef{File: link.File, Key: link.Key}] = true
	}

	var errs []string
	seen := map[domain.EnvKeyRef]bool{}
	for _, link := range cfg.EnvValues {
		ref := domain.EnvKeyRef{File: link.File, Key: link.Key}
		if link.File == "" {
			errs = append(errs, fmt.Sprintf(domain.EnvValueLinkFileRequiredFmt, link.Key))
		}
		if !IsEnvVarName(link.Key) {
			errs = append(errs, fmt.Sprintf(domain.EnvValueLinkBadKeyFmt, link.File, link.Key))
		}
		if link.Value == "" {
			errs = append(errs, fmt.Sprintf(domain.EnvValueLinkValueEmptyFmt, link.Key, link.File))
		}
		if seen[ref] {
			errs = append(errs, fmt.Sprintf(domain.EnvValueLinkTwiceFmt, link.Key, link.File))
		}
		seen[ref] = true
		if ports[ref] {
			errs = append(errs, fmt.Sprintf(domain.EnvValueLinkClashesPortFmt, link.Key, link.File))
		}

		job, found := byName[link.Job]
		if !found {
			errs = append(errs, fmt.Sprintf(domain.EnvValueLinkNoJobFmt, link.Key, link.File, link.Job))
			continue
		}
		errs = append(errs, envValueTemplateErrors(link, job)...)
	}
	return errs
}

// envValueTemplateErrors expands the template against a stand-in worktree, which
// is how a placeholder nobody defined is caught at load rather than the first
// time a worktree is created. The ports and the origin are the only parts that
// vary per worktree, and neither changes which placeholders resolve.
func envValueTemplateErrors(link domain.EnvValueLink, job domain.JobConfig) []string {
	_, err := ExpandEnvValue(ExpandEnvValueParams{
		Link:     link,
		Job:      job,
		Worktree: domain.NamespaceProbeWorktree,
		Shared:   IsShared(job),
		Origin:   envValueProbeOrigin(job),
	})
	if err == nil {
		return nil
	}
	return []string{err.Error()}
}

// envValueProbeOrigin stands in for an address at load: whether the machine
// serves names is not run.toml's business, and refusing a template here for a
// proxy that happens to be off would refuse the config on one machine and not
// on another.
func envValueProbeOrigin(job domain.JobConfig) string {
	if job.URL == nil {
		return ""
	}
	return domain.NamespaceProbeWorktree
}

// envPortLinkError names the jobs that do declare the port, so the fix is the
// line to write rather than a hunt through run.toml.
func envPortLinkError(bases map[domain.PortRef]int, link domain.EnvPortLink) string {
	var jobs []string
	for _, ref := range sortedPortRefs(bases) {
		if ref.Name == link.Port {
			jobs = append(jobs, ref.Job)
		}
	}

	if len(jobs) == 0 {
		return fmt.Sprintf("env_port %s in %s references port %q, which no job declares", link.Key, link.File, link.Port)
	}
	if link.Job == "" {
		return fmt.Sprintf("env_port %s in %s: job is required — %q is declared by %s",
			link.Key, link.File, link.Port, strings.Join(jobs, ", "))
	}
	return fmt.Sprintf("env_port %s in %s references port %q of job %q, which does not declare it — %s does",
		link.Key, link.File, link.Port, link.Job, strings.Join(jobs, ", "))
}

// ValidateEnvPortTargets is the half of the link check that needs both configs:
// a link may only name a .env the project actually provisions, otherwise wtm
// would promise to rewrite a file nothing ever creates.
func ValidateEnvPortTargets(links []domain.EnvPortLink, files []domain.EnvFile) []string {
	targets := make(map[string]bool, len(files))
	for _, f := range files {
		targets[f.Target] = true
	}

	var errs []string
	for _, link := range links {
		if !targets[link.File] {
			errs = append(errs, fmt.Sprintf("env_port %s references %s, which is not a configured env file — add it to [env] in %s or drop the link", link.Key, link.File, domain.ConfigFileName))
		}
	}
	return errs
}
