package runinit

import (
	"cmp"
	"encoding/json"
	"maps"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/dotenv"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// ----------------------------------------------------------------------------
// Facts

// Facts are what run init decides from, as Observe read them: the repository's name, its worktrees (the main one first) with their ordinals, the proxy's port; run.toml and config.toml as written; its docker compose file, its package manifests, and every .env file a worktree may hold, config.toml's and the repository's, with the lines each holds or its template does.
type Facts struct {
	Project   string
	Worktrees []Previewable
	ProxyPort int
	Run       project.SavedRun
	Config    *project.Saved
	Compose   project.Compose
	Workspace project.Workspace
	EnvFiles  []project.EnvFile
	EnvLines  map[string][]dotenv.Line
}

// Previewable is a worktree the plan may preview: its branch (the main one's for the main worktree), its ordinal (0 for the main worktree or one never numbered), and whether it is the main one.
type Previewable struct {
	Branch  string
	Ordinal int
	Main    bool
}

// ----------------------------------------------------------------------------
// The candidates

// A Candidate is a job run init may write: a service of docker-compose.yml, an app's dev or start script, a package's other script; its name, the folder it runs in (none for a compose service), its script, the command and stop command it is written with, its kind, its image (a compose service's), whether it is built here, whether a first init checks it, and whether run.toml declares it.
type Candidate struct {
	Name     string          `json:"name"`
	Place    string          `json:"place"`
	Script   string          `json:"script,omitempty"`
	Cmd      string          `json:"cmd"`
	Stop     string          `json:"stop,omitempty"`
	Kind     project.JobKind `json:"kind"`
	Image    string          `json:"image,omitempty"`
	Compose  bool            `json:"compose"`
	Build    bool            `json:"build,omitempty"`
	Proposed bool            `json:"proposed"`
	OnDisk   bool            `json:"on_disk"`
}

// notTasks are the scripts of a package that are never offered as a job: they build, check or install, they do not run the project.
var notTasks = map[string]bool{
	"build": true, "test": true, "lint": true, "typecheck": true, "check": true, "format": true, "clean": true,
	"prepare": true, "preinstall": true, "install": true, "postinstall": true, "prepublishOnly": true,
}

// runners are the tools a root script starts several apps through.
var runners = regexp.MustCompile(`\b(turbo|nx|lerna)\b|pnpm (-r|--recursive)|npm run \S+ (-ws|--workspaces)`)

// runnerName is the name of the job a root script that starts the apps is written as, through it.
const runnerName = "dev"

// candidates are every job run init may write, in the order a surface lists them: docker-compose.yml's services, then each place's script jobs; and the root script that starts the apps, when there is one.
func (f Facts) candidates() ([]Candidate, *Candidate) {
	var found []Candidate
	declared := func(name string) bool {
		return slices.ContainsFunc(f.Run.Run.Jobs, func(job project.Job) bool { return job.Name == name })
	}
	for _, service := range f.Compose.Services {
		found = append(found, Candidate{
			Name: service.Name, Cmd: "docker compose up -d " + service.Name, Stop: "docker compose stop " + service.Name,
			Kind: project.Service, Image: service.Image, Compose: true, Build: service.Build, Proposed: true,
		})
	}

	var runner *Candidate
	taken := map[string]bool{}
	for _, c := range found {
		taken[c.Name] = true
	}
	for _, place := range f.Workspace.Places {
		for _, c := range f.placeCandidates(place) {
			if place.Dir == "." && runners.MatchString(scriptOf(place, c.Script)) && len(f.Workspace.Places) > 1 {
				runner = &c
				runner.Name = runnerName
				continue
			}
			if taken[c.Name] {
				c.Name = worktree.Slug(place.Name) + "-" + c.Script
			}
			taken[c.Name] = true
			found = append(found, c)
		}
	}
	for i := range found {
		if job, ok := f.declaredAs(found[i]); ok {
			found[i].Name = job.Name
		}
		found[i].OnDisk = declared(found[i].Name)
	}
	if runner != nil {
		runner.OnDisk = declared(runner.Name)
	}
	return found, runner
}

// placeCandidates are the jobs of one place: its dev script, else its start script, as a service named after the place; a place with neither, each script that runs something (not notTasks) as a task named after the script.
func (f Facts) placeCandidates(place project.Place) []Candidate {
	for _, name := range []string{"dev", "start"} {
		if cmd := scriptOf(place, name); cmd != "" {
			return []Candidate{{Name: worktree.Slug(place.Name), Place: place.Dir, Script: name, Cmd: f.run(name), Kind: project.Service, Proposed: true}}
		}
	}
	if place.Dir == "." {
		return nil
	}

	var tasks []Candidate
	for _, script := range place.Scripts {
		if notTasks[script.Name] || strings.HasPrefix(script.Name, "pre") || strings.HasPrefix(script.Name, "post") {
			continue
		}
		tasks = append(tasks, Candidate{Name: worktree.Slug(script.Name), Place: place.Dir, Script: script.Name, Cmd: f.run(script.Name), Kind: project.Task, Proposed: true})
	}
	return tasks
}

// declaredAs is the job run.toml declares for a script candidate under another name: the job that runs its script in its folder (the legacy wtm named them api-dev).
func (f Facts) declaredAs(c Candidate) (project.Job, bool) {
	if c.Compose {
		return project.Job{}, false
	}
	cwd := c.Place
	if cwd == "." {
		cwd = ""
	}
	for _, job := range f.Run.Run.Jobs {
		if job.Name != c.Name && strings.TrimPrefix(job.Cwd, "./") == cwd && (strings.HasSuffix(job.Cmd, " "+c.Script) || job.Cmd == c.Cmd) {
			return job, true
		}
	}
	return project.Job{}, false
}

func scriptOf(place project.Place, name string) string {
	if at := slices.IndexFunc(place.Scripts, func(s project.Script) bool { return s.Name == name }); at >= 0 {
		return place.Scripts[at].Cmd
	}
	return ""
}

// run is the command that runs a script with the repository's package manager: npm's run, the others' own (pnpm dev).
func (f Facts) run(script string) string {
	if manager := f.Workspace.Manager; manager != "" && manager != "npm" {
		return manager + " " + script
	}
	return "npm run " + script
}

// exec is the command that runs a script's own command line with the repository's package manager, its binaries on the path.
func (f Facts) exec(line string) string {
	switch manager := f.Workspace.Manager; manager {
	case "", "npm":
		return "npm exec -- " + line
	case "bun":
		return "bunx " + line
	default:
		return manager + " exec " + line
	}
}

// ----------------------------------------------------------------------------
// The decision of each part

// A Decision is run init's request settled on the facts, part by part, with where each answer came from (Origins, by path) and why a field is not asked (Skips, by path).
type Decision struct {
	Candidates []Candidate
	Runnable   *Candidate
	Picked     []string
	Tasks      []string
	Runner     Runner

	Shareable      []string
	Shared         []string
	Spaces         map[string]project.Namespace
	ProposedSpaces map[string]project.Namespace
	Touches        []string

	Ports         []Port
	ProposedPorts []Port
	Commands      map[string]string
	Rewrites      map[string]string

	Addressing  project.Addressing
	Addressable []string
	URLs        []string

	Linkable []Link
	Links    []Link
	Uncopied []string
	EnvCopy  []string

	Profiles         []project.Profile
	ProposedProfiles []project.Profile
	DefaultProfile   string

	PatchCompose bool
	Previewable  []string
	Preview      string

	Origins map[string]domain.Origin
	Skips   map[string]domain.Skip
	given   Request
}

// A Port is a job's port as run.toml declares it: the variable the job reads it from and its base, on the main worktree; no variable for a job that binds none. From says where it was found: docker-compose.yml, an .env file's path, the job's script, run.toml or the request.
type Port struct {
	Job      string `json:"job"`
	Variable string `json:"variable,omitempty"`
	Base     int    `json:"base,omitempty"`
	From     string `json:"from,omitempty"`
}

// LinkKind is what a linked .env key holds of its job: its port, its address, or its data (a URL to its namespace).
type LinkKind string

// The kinds of link.
const (
	LinkPort    LinkKind = "port"
	LinkAddress LinkKind = "address"
	LinkData    LinkKind = "data"
)

// A Link is an .env key that follows a job: its file and key, its value as written, the job and the port it follows, what it holds, and for data the template run.toml writes it from.
type Link struct {
	File     string   `json:"file"`
	Key      string   `json:"key"`
	Value    string   `json:"value"`
	Job      string   `json:"job"`
	Port     string   `json:"port"`
	Kind     LinkKind `json:"kind"`
	Template string   `json:"template,omitempty"`
}

// decide settles every part of the request, each on the parts before it.
func (f Facts) decide(req Request) Decision {
	d := Decision{Origins: map[string]domain.Origin{}, Skips: map[string]domain.Skip{}, Commands: map[string]string{}, Rewrites: map[string]string{}, given: req}
	d.Candidates, d.Runnable = f.candidates()
	f.decideRuns(&d, req)
	f.decideShared(&d, req)
	f.decideCompose(&d, req)
	f.decidePorts(&d, req)
	f.decideAddresses(&d, req)
	f.decideLinks(&d, req)
	f.decideProfiles(&d, req)
	f.decidePreview(&d, req)
	return d
}

// declared reports whether run.toml exists and so answers each part: a re-init.
func (f Facts) declared() bool {
	return f.Run.Run.Declared
}

// settle is the answer of the field at path: given, else as run.toml holds it on a re-init, else proposed.
func settle[T any](d *Decision, at string, given T, isGiven bool, onDisk func() T, proposed T, reinit bool) T {
	switch {
	case isGiven:
		d.Origins[at] = domain.OriginRequest
		return given
	case reinit:
		d.Origins[at] = domain.OriginConfig
		return onDisk()
	}
	d.Origins[at] = domain.OriginDetected
	return proposed
}

func (f Facts) decideRuns(d *Decision, req Request) {
	var proposed, onDisk, tasksProposed, tasksOnDisk []string
	for _, c := range d.Candidates {
		if c.Proposed {
			proposed = append(proposed, c.Name)
		}
		if c.OnDisk {
			onDisk = append(onDisk, c.Name)
		}
		if c.Kind == project.Task {
			tasksProposed = append(tasksProposed, c.Name)
		}
		if job, ok := f.job(c.Name); ok && job.Kind == project.Task {
			tasksOnDisk = append(tasksOnDisk, c.Name)
		}
	}
	d.Picked = settle(d, PathJobs, req.Jobs, req.Jobs != nil, func() []string { return onDisk }, proposed, f.declared())
	tasks := settle(d, PathTasks, req.Tasks, req.Tasks != nil, func() []string { return tasksOnDisk }, tasksProposed, f.declared())
	d.Tasks = slices.DeleteFunc(slices.Clone(tasks), func(name string) bool { return !slices.Contains(d.Picked, name) })
	if len(d.Candidates) == 0 {
		d.Skips[PathJobs], d.Skips[PathTasks] = domain.SkipNothingToAsk, domain.SkipNothingToAsk
	}

	if d.Runnable == nil {
		d.Runner, d.Skips[PathRunner] = OneByOne, domain.SkipNoRunner
		return
	}
	through := Through
	if !d.Runnable.OnDisk {
		through = OneByOne
	}
	d.Runner = settle(d, PathRunner, req.Runner, req.Runner != "", func() Runner { return through }, OneByOne, f.declared())
}

// TaskNames are the jobs picked that run once.
func (d Decision) TaskNames() []string {
	return d.Tasks
}

// candidate is the candidate of name.
func (d Decision) candidate(name string) (Candidate, bool) {
	if at := slices.IndexFunc(d.Candidates, func(c Candidate) bool { return c.Name == name }); at >= 0 {
		return d.Candidates[at], true
	}
	if d.Runnable != nil && d.Runnable.Name == name {
		return *d.Runnable, true
	}
	return Candidate{}, false
}

// services are the jobs picked that keep running, in the candidates' order.
func (d Decision) services() []string {
	return slices.DeleteFunc(slices.Clone(d.Picked), func(name string) bool { return slices.Contains(d.Tasks, name) })
}

// sharedImages are the images whose service one copy serves best for every worktree: a database, which each worktree gets its own space in, or a mail catcher.
var sharedImages = map[string]bool{"postgres": true, "postgis": true, "mysql": true, "mariadb": true, "mongo": true, "mailpit": true, "mailhog": true}

// imageName is an image's name without its registry, owner and tag: axllent/mailpit:latest is mailpit.
func imageName(image string) string {
	name, _, _ := strings.Cut(path.Base(image), ":")
	return name
}

func (f Facts) decideShared(d *Decision, req Request) {
	var proposed, onDisk []string
	for _, name := range d.services() {
		c, _ := d.candidate(name)
		if !c.Compose || c.Build {
			continue
		}
		d.Shareable = append(d.Shareable, name)
		if sharedImages[imageName(c.Image)] {
			proposed = append(proposed, name)
		}
		if job, ok := f.job(name); ok && job.Scope == project.Shared {
			onDisk = append(onDisk, name)
		}
	}
	if f.Compose.File == "" {
		d.Skips[PathShared] = domain.SkipNoCompose
	}
	d.Shared = slices.DeleteFunc(slices.Clone(settle(d, PathShared, req.Shared, req.Shared != nil, func() []string { return onDisk }, proposed, f.declared())), func(name string) bool {
		return !slices.Contains(d.Shareable, name)
	})

	d.ProposedSpaces, d.Spaces = map[string]project.Namespace{}, map[string]project.Namespace{}
	spacesOnDisk := map[string]project.Namespace{}
	for _, name := range d.Shareable {
		d.ProposedSpaces[name] = f.space(name)
		if job, ok := f.job(name); ok && job.Namespace != nil {
			spacesOnDisk[name] = *job.Namespace
		}
	}
	d.Spaces = settle(d, PathSpaces, parseSpaces(req.Spaces), req.Spaces != nil, func() map[string]project.Namespace { return spacesOnDisk }, d.ProposedSpaces, f.declared())
	if len(d.Shared) == 0 {
		d.Skips[PathSpaces] = domain.SkipNothingToAsk
	}

	var touches []string
	for _, task := range d.Tasks {
		if c, _ := d.candidate(task); c.Kind == project.Task && touchesData(c.Script) {
			touches = append(touches, task)
		}
	}
	var touchesOnDisk []string
	for _, task := range d.Tasks {
		if job, ok := f.job(task); ok && len(job.Touches) > 0 {
			touchesOnDisk = append(touchesOnDisk, task)
		}
	}
	d.Touches = slices.DeleteFunc(slices.Clone(settle(d, PathTouches, req.Touches, req.Touches != nil, func() []string { return touchesOnDisk }, touches, f.declared())), func(name string) bool {
		return !slices.Contains(d.Tasks, name)
	})
	if len(d.Tasks) == 0 || !slices.ContainsFunc(d.Shared, func(name string) bool { return d.Spaces[name].Name != "" }) {
		d.Skips[PathTouches] = domain.SkipNothingToAsk
	}
}

// parseSpaces are the spaces a request spells service.part=value; nil for none given.
func parseSpaces(spelled []string) map[string]project.Namespace {
	if spelled == nil {
		return nil
	}
	spaces := map[string]project.Namespace{}
	for _, entry := range spelled {
		at, value, _ := strings.Cut(entry, "=")
		dot := strings.LastIndex(at, ".")
		if dot <= 0 {
			continue
		}
		service, part := at[:dot], at[dot+1:]
		space := spaces[service]
		switch part {
		case SpaceName:
			space.Name = value
		case SpaceCreate:
			space.Create = value
		case SpaceRemove:
			space.Remove = value
		}
		spaces[service] = space
	}
	return spaces
}

// dataScripts are the words of a script that changes the data of a database.
var dataScripts = regexp.MustCompile(`(?i)migrat|seed|reset|fixture|db[:_-]`)

func touchesData(script string) bool {
	return dataScripts.MatchString(script)
}

// space is what each worktree gets of a shared service as the detection fills it: a database of its own named after the project and the worktree for a database server, made and dropped through docker compose; nothing for any other.
func (f Facts) space(service string) project.Namespace {
	c := ""
	for _, s := range f.Compose.Services {
		if s.Name == service {
			c = imageName(s.Image)
		}
	}
	name := strings.ReplaceAll(worktree.Slug(f.Project), "-", "_") + "_{worktree}"
	exec := "docker compose exec -T " + service + " "
	switch c {
	case "postgres", "postgis":
		return project.Namespace{Name: name, Create: exec + `createdb -U postgres "$` + namespaceKey + `"`, Remove: exec + `dropdb -U postgres --if-exists "$` + namespaceKey + `"`}
	case "mysql", "mariadb":
		return project.Namespace{Name: name, Create: exec + `mysql -uroot -e "CREATE DATABASE IF NOT EXISTS \` + "`$" + namespaceKey + "`" + `"`, Remove: exec + `mysql -uroot -e "DROP DATABASE IF EXISTS \` + "`$" + namespaceKey + "`" + `"`}
	}
	return project.Namespace{}
}

// namespaceKey is the variable a namespace's commands read its name from (env.NamespaceKey).
const namespaceKey = "WTM_NAMESPACE"

// ----------------------------------------------------------------------------
// The decision of the ports

func (f Facts) decideCompose(d *Decision, req Request) {
	d.PatchCompose = true
	d.Origins[PathPatchCompose] = domain.OriginDefault
	if req.PatchCompose != nil {
		d.PatchCompose, d.Origins[PathPatchCompose] = *req.PatchCompose, domain.OriginRequest
	}
	if len(d.composeEdits(f)) == 0 {
		d.Skips[PathPatchCompose] = domain.SkipNothingToAsk
	}
}

var (
	literalPort  = regexp.MustCompile(`^\d{2,5}$`)
	variablePort = regexp.MustCompile(`^\$\{([A-Za-z_][A-Za-z0-9_]*)(?::?-(\d{2,5}))?\}$`)
	scriptPort   = regexp.MustCompile(`(?:(?:^|\s)(?:-p|--port)(?:\s+|=)|\bPORT=)(\d{2,5})\b`)
	variableName = regexp.MustCompile(`[^A-Z0-9]+`)
	nonHTTPPorts = map[string]bool{"5432": true, "3306": true, "6379": true, "27017": true, "1025": true, "11211": true, "5672": true, "9092": true, "2181": true}
	portInValue  = regexp.MustCompile(`(?:^|:)(\d{2,5})(?:$|[/?#])`)
)

// envName is s as an environment variable's name: upper case, any other character an underscore.
func envName(s string) string {
	return strings.Trim(variableName.ReplaceAllString(strings.ToUpper(s), "_"), "_")
}

// proposedPorts are the ports the repository suggests for the services picked, in their order: a compose service's published ports, its literal ones only when docker-compose.yml is rewritten to read a variable; a script's PORT in its folder's .env files, else the port its script binds literally, under JOB_PORT; none for a service neither says.
func (f Facts) proposedPorts(d *Decision) []Port {
	taken := map[string]bool{}
	for _, read := range f.Compose.Reads {
		taken[read] = true
	}
	var ports []Port
	for _, name := range d.services() {
		c, known := d.candidate(name)
		if !known {
			continue
		}
		if c.Compose {
			ports = append(ports, f.composePorts(composePortsParams{Service: name, Patch: d.PatchCompose, Taken: taken})...)
			continue
		}
		port := Port{Job: name}
		if variable, base, file, ok := f.envPort(c.Place); ok {
			port.Variable, port.Base, port.From = variable, base, file
		} else if match := scriptPort.FindStringSubmatch(scriptOf(f.place(c.Place), c.Script)); match != nil {
			port.Variable, port.From = envName(name)+"_PORT", path.Join(c.Place, "package.json")
			port.Base, _ = strconv.Atoi(match[1])
		}
		ports = append(ports, port)
	}
	return ports
}

type composePortsParams struct {
	Service string
	Patch   bool
	Taken   map[string]bool
}

// composePorts are the ports a compose service publishes as run.toml declares them: a variable its host side reads, with its default as base, as it stands; a literal under SERVICE_PORT (then SERVICE_PORT_<container>) when the file is rewritten to read it.
func (f Facts) composePorts(params composePortsParams) []Port {
	var ports []Port
	for _, service := range f.Compose.Services {
		if service.Name != params.Service {
			continue
		}
		for _, published := range service.Ports {
			if published.Unreadable != "" {
				continue
			}
			if match := variablePort.FindStringSubmatch(published.Host); match != nil {
				base, _ := strconv.Atoi(match[2])
				ports = append(ports, Port{Job: service.Name, Variable: match[1], Base: base, From: f.Compose.File})
				params.Taken[match[1]] = true
				continue
			}
			if !literalPort.MatchString(published.Host) || !params.Patch {
				continue
			}
			base, _ := strconv.Atoi(published.Host)
			variable := envName(service.Name) + "_PORT"
			if params.Taken[variable] {
				variable += "_" + envName(published.Container)
			}
			params.Taken[variable] = true
			ports = append(ports, Port{Job: service.Name, Variable: variable, Base: base, From: f.Compose.File})
		}
	}
	if len(ports) == 0 {
		ports = append(ports, Port{Job: params.Service})
	}
	return ports
}

// place is the place at dir.
func (f Facts) place(dir string) project.Place {
	if at := slices.IndexFunc(f.Workspace.Places, func(p project.Place) bool { return p.Dir == dir }); at >= 0 {
		return f.Workspace.Places[at]
	}
	return project.Place{Dir: dir}
}

// envPort is the PORT an .env file of the folder dir sets, a file before its template: its variable, base and file.
func (f Facts) envPort(dir string) (string, int, string, bool) {
	for _, file := range f.EnvFiles {
		if path.Dir(file.Target) != path.Clean(dir) {
			continue
		}
		for _, source := range []string{file.Target, file.Template} {
			for _, line := range f.EnvLines[source] {
				if base, err := strconv.Atoi(line.Value); line.Kind == dotenv.Pair && line.Key == "PORT" && err == nil && source != "" {
					return line.Key, base, source, true
				}
			}
		}
	}
	return "", 0, "", false
}

func (f Facts) decidePorts(d *Decision, req Request) {
	d.ProposedPorts = f.proposedPorts(d)
	var onDisk []Port
	for _, name := range d.services() {
		job, ok := f.job(name)
		if !ok {
			onDisk = append(onDisk, portsOf(d.ProposedPorts, name)...)
			continue
		}
		if len(job.Ports) == 0 {
			onDisk = append(onDisk, Port{Job: name, From: runFile})
		}
		for _, variable := range slices.Sorted(maps.Keys(job.Ports)) {
			onDisk = append(onDisk, Port{Job: name, Variable: variable, Base: job.Ports[variable], From: runFile})
		}
	}
	given := parsePorts(req.Ports)
	d.Ports = settle(d, PathPorts, given, req.Ports != nil, func() []Port { return onDisk }, d.ProposedPorts, f.declared())

	for _, port := range d.Ports {
		c, ok := d.candidate(port.Job)
		if !ok || c.Compose || port.Variable == "" {
			continue
		}
		script := scriptOf(f.place(c.Place), c.Script)
		if match := scriptPort.FindStringSubmatchIndex(script); match != nil && script[match[2]:match[3]] == strconv.Itoa(port.Base) {
			d.Rewrites[port.Job] = f.exec(script[:match[2]] + "${" + port.Variable + "}" + script[match[3]:])
		}
	}
	commandsOnDisk := map[string]string{}
	for _, c := range d.Candidates {
		if job, ok := f.job(c.Name); ok && job.Cmd != c.Cmd {
			commandsOnDisk[c.Name] = job.Cmd
		}
	}
	d.Commands = settle(d, PathCommands, parseCommands(req.Commands), req.Commands != nil, func() map[string]string { return commandsOnDisk }, map[string]string{}, f.declared())
	if len(d.Picked) == 0 {
		d.Skips[PathPorts] = domain.SkipNothingToAsk
	}
}

func portsOf(ports []Port, job string) []Port {
	return slices.DeleteFunc(slices.Clone(ports), func(p Port) bool { return p.Job != job })
}

// parsePorts are the ports a request spells job.VARIABLE=base, or job= for none; nil for none given.
func parsePorts(spelled []string) []Port {
	if spelled == nil {
		return nil
	}
	ports := []Port{}
	for _, entry := range spelled {
		left, base, _ := strings.Cut(entry, "=")
		job, variable, _ := strings.Cut(left, ".")
		port := Port{Job: job, Variable: variable, From: fromRequest}
		port.Base, _ = strconv.Atoi(base)
		ports = append(ports, port)
	}
	return ports
}

func parseCommands(spelled []string) map[string]string {
	commands := map[string]string{}
	for _, entry := range spelled {
		job, cmd, _ := strings.Cut(entry, "=")
		commands[job] = cmd
	}
	return commands
}

const (
	runFile     = "run.toml"
	fromRequest = "request"
)

// ----------------------------------------------------------------------------
// The decision of the addresses

func (f Facts) decideAddresses(d *Decision, req Request) {
	addressing := cmp.Or(f.Run.Run.Addressing, project.AddressNames)
	d.Addressing = settle(d, PathAddressing, req.Addressing, req.Addressing != "", func() project.Addressing { return addressing }, project.AddressNames, f.declared())

	var proposed, onDisk []string
	for _, name := range d.services() {
		port, ok := d.firstPort(name)
		if !ok {
			continue
		}
		c, _ := d.candidate(name)
		if c.Compose && !f.http(name, port) {
			continue
		}
		d.Addressable = append(d.Addressable, name)
		if !c.Compose {
			proposed = append(proposed, name)
		}
		if job, ok := f.job(name); ok && job.URL != nil {
			onDisk = append(onDisk, name)
		}
	}
	d.URLs = slices.DeleteFunc(slices.Clone(settle(d, PathURLs, req.URLs, req.URLs != nil, func() []string { return onDisk }, proposed, f.declared())), func(name string) bool {
		return !slices.Contains(d.Addressable, name)
	})
	if len(d.Addressable) == 0 {
		d.Skips[PathURLs] = domain.SkipNothingToAsk
	}
}

// firstPort is the first port job declares.
func (d Decision) firstPort(job string) (Port, bool) {
	at := slices.IndexFunc(d.Ports, func(p Port) bool { return p.Job == job && p.Variable != "" })
	if at < 0 {
		return Port{}, false
	}
	return d.Ports[at], true
}

// http reports whether a compose service's port speaks HTTP: any but a database's, a cache's, a broker's or SMTP's.
func (f Facts) http(service string, port Port) bool {
	for _, s := range f.Compose.Services {
		for _, published := range s.Ports {
			if s.Name == service && strings.Contains(published.Host, strconv.Itoa(port.Base)) {
				return !nonHTTPPorts[published.Container]
			}
		}
	}
	return true
}

// ----------------------------------------------------------------------------
// The decision of the .env files

// linkable are the .env keys that follow a job, file by file in the order config.toml and the repository list them: a value holding a port a job declares, a shared service's only through its own space (its port never moves); a URL to a shared service's database is its data, written from a template; a URL to a job answering under its name, its address; any other, its port.
func (f Facts) linkable(d *Decision) []Link {
	bases := map[int]Port{}
	for _, port := range d.Ports {
		if port.Variable != "" {
			if _, seen := bases[port.Base]; !seen {
				bases[port.Base] = port
			}
		}
	}
	var links []Link
	for _, file := range f.EnvFiles {
		source := file.Target
		if _, ok := f.EnvLines[source]; !ok {
			source = file.Template
		}
		for _, line := range f.EnvLines[source] {
			if line.Kind != dotenv.Pair {
				continue
			}
			if link, ok := linkOf(linkParams{File: file.Target, Line: line, Bases: bases, Decision: d}); ok {
				links = append(links, link)
			}
		}
	}
	return links
}

type linkParams struct {
	File     string
	Line     dotenv.Line
	Bases    map[int]Port
	Decision *Decision
}

func linkOf(params linkParams) (Link, bool) {
	match := portInValue.FindStringSubmatchIndex(params.Line.Value)
	if match == nil {
		return Link{}, false
	}
	value := params.Line.Value
	base, _ := strconv.Atoi(value[match[2]:match[3]])
	port, found := params.Bases[base]
	if !found {
		return Link{}, false
	}
	d := params.Decision
	link := Link{File: params.File, Key: params.Line.Key, Value: value, Job: port.Job, Port: port.Variable, Kind: LinkPort}
	space := d.Spaces[port.Job]
	switch shared := slices.Contains(d.Shared, port.Job); {
	case shared && space.Name != "" && strings.Contains(value, "://"):
		link.Kind = LinkData
		link.Template = dataTemplate(value, match, port.Variable)
	case shared:
		return Link{}, false
	case strings.Contains(value, "://") && slices.Contains(d.URLs, port.Job):
		link.Kind = LinkAddress
	}
	return link, true
}

// dataTemplate is a URL to a database written to follow its job: its port {port.VARIABLE}, its database {namespace}.
func dataTemplate(value string, match []int, variable string) string {
	templated := value[:match[2]] + "{port." + variable + "}" + value[match[3]:]
	scheme, rest, _ := strings.Cut(templated, "://")
	host, _, hasPath := strings.Cut(rest, "/")
	if !hasPath {
		return templated + "/{namespace}"
	}
	query := ""
	if at := strings.IndexAny(rest[len(host)+1:], "?#"); at >= 0 {
		query = rest[len(host)+1+at:]
	}
	return scheme + "://" + host + "/{namespace}" + query
}

func (f Facts) decideLinks(d *Decision, req Request) {
	d.Linkable = f.linkable(d)
	var onDisk []Link
	for _, link := range d.Linkable {
		linked := slices.ContainsFunc(f.Run.Run.EnvPorts, func(l project.EnvPortLink) bool { return l.File == link.File && l.Key == link.Key }) ||
			slices.ContainsFunc(f.Run.Run.EnvValues, func(l project.EnvValueLink) bool { return l.File == link.File && l.Key == link.Key })
		if linked {
			onDisk = append(onDisk, link)
		}
	}
	var given []Link
	for _, spelled := range req.Links {
		if at := slices.IndexFunc(d.Linkable, func(l Link) bool { return l.Spelled() == spelled }); at >= 0 {
			given = append(given, d.Linkable[at])
		} else {
			file, key, _ := strings.Cut(spelled, "#")
			given = append(given, Link{File: file, Key: key})
		}
	}
	d.Links = settle(d, PathLinks, given, req.Links != nil, func() []Link { return onDisk }, d.Linkable, f.declared())
	if len(d.Linkable) == 0 {
		d.Skips[PathLinks] = domain.SkipNothingToAsk
	}

	for _, link := range d.Linkable {
		if !f.copied(link.File) && !slices.Contains(d.Uncopied, link.File) {
			d.Uncopied = append(d.Uncopied, link.File)
		}
	}
	d.EnvCopy = slices.DeleteFunc(slices.Clone(settle(d, PathEnvCopy, req.EnvCopy, req.EnvCopy != nil, func() []string { return []string{} }, []string{}, false)), func(file string) bool {
		return !slices.Contains(d.Uncopied, file)
	})
	if len(d.Uncopied) == 0 {
		d.Skips[PathEnvCopy] = domain.SkipNothingToAsk
	}
}

// copied reports whether config.toml copies the .env file target into a new worktree.
func (f Facts) copied(target string) bool {
	return f.Config != nil && slices.ContainsFunc(f.Config.Config.Env.Files, func(file project.EnvFile) bool { return file.Target == target })
}

// ----------------------------------------------------------------------------
// The decision of the profiles

// proposedProfiles are the profiles a first init proposes: all, every job picked (through the runner, the runner instead of the apps it starts), the default; then one per app, the app and the jobs its .env keys reach.
func (d Decision) proposedProfiles() []project.Profile {
	all := project.Profile{Name: "all"}
	for _, name := range d.Picked {
		if d.Runner == Through && d.startedByRunner(name) {
			continue
		}
		all.Jobs = append(all.Jobs, name)
	}
	if d.Runner == Through {
		all.Jobs = append(all.Jobs, runnerName)
	}
	if len(all.Jobs) == 0 {
		return nil
	}
	profiles := []project.Profile{all}
	for _, name := range d.services() {
		c, _ := d.candidate(name)
		if c.Compose {
			continue
		}
		profiles = append(profiles, project.Profile{Name: name, Jobs: d.needs(name)})
	}
	return profiles
}

// needs are the jobs a job needs to run, in the order they start, itself last: the jobs its folder's .env keys follow, and theirs.
func (d Decision) needs(name string) []string {
	var order []string
	seen := map[string]bool{}
	var visit func(string)
	visit = func(job string) {
		if seen[job] {
			return
		}
		seen[job] = true
		c, _ := d.candidate(job)
		for _, link := range d.Links {
			if !c.Compose && path.Dir(link.File) == path.Clean(c.Place) {
				visit(link.Job)
			}
		}
		order = append(order, job)
	}
	visit(name)
	return order
}

// startedByRunner reports whether the runner starts the job of name: an app's service.
func (d Decision) startedByRunner(name string) bool {
	c, ok := d.candidate(name)
	return ok && !c.Compose && c.Kind == project.Service && c.Place != "." && !slices.Contains(d.Tasks, name)
}

func (f Facts) decideProfiles(d *Decision, req Request) {
	d.ProposedProfiles = d.proposedProfiles()
	onDisk := slices.Clone(f.Run.Run.Profiles)
	given := []project.Profile{}
	for _, spelled := range req.Profiles {
		name, jobs, _ := strings.Cut(spelled, "=")
		given = append(given, project.Profile{Name: strings.TrimSpace(name), Jobs: filled(strings.Split(jobs, ","))})
	}
	d.Profiles = settle(d, PathProfiles, given, req.Profiles != nil, func() []project.Profile { return onDisk }, d.ProposedProfiles, f.declared())
	left := func(job string) bool {
		_, candidate := d.candidate(job)
		return candidate && !slices.Contains(d.Picked, job) && job != runnerName
	}
	// A profile goes with the job it is named after, and with its last job.
	d.Profiles = slices.DeleteFunc(slices.Clone(d.Profiles), func(profile project.Profile) bool { return left(profile.Name) })
	for i, profile := range d.Profiles {
		d.Profiles[i].Default = false
		d.Profiles[i].Jobs = slices.DeleteFunc(slices.Clone(profile.Jobs), left)
	}
	d.Profiles = slices.DeleteFunc(d.Profiles, func(profile project.Profile) bool { return len(profile.Jobs) == 0 })

	fallback := ""
	if len(d.Profiles) > 0 {
		fallback = d.Profiles[0].Name
	}
	defaultOnDisk := fallback
	if at := slices.IndexFunc(f.Run.Run.Profiles, func(p project.Profile) bool { return p.Default }); at >= 0 {
		defaultOnDisk = f.Run.Run.Profiles[at].Name
	}
	d.DefaultProfile = settle(d, PathDefaultProfile, req.DefaultProfile, req.DefaultProfile != "", func() string { return defaultOnDisk }, fallback, f.declared())
	for i := range d.Profiles {
		d.Profiles[i].Default = d.Profiles[i].Name == d.DefaultProfile
	}
	if len(d.Profiles) == 0 {
		d.Skips[PathDefaultProfile] = domain.SkipNoProfile
	}
}

// ----------------------------------------------------------------------------
// The decision of the preview

func (f Facts) decidePreview(d *Decision, req Request) {
	fallback := ""
	for _, w := range f.Worktrees {
		d.Previewable = append(d.Previewable, w.Branch)
		if fallback == "" && !w.Main {
			fallback = w.Branch
		}
	}
	if fallback == "" && len(f.Worktrees) > 0 {
		fallback = f.Worktrees[0].Branch
	}
	d.Preview, d.Origins[PathPreview] = fallback, domain.OriginDefault
	if req.Preview != "" {
		d.Preview, d.Origins[PathPreview] = req.Preview, domain.OriginRequest
	}
}

// job is the job run.toml declares under name.
func (f Facts) job(name string) (project.Job, bool) {
	if at := slices.IndexFunc(f.Run.Run.Jobs, func(job project.Job) bool { return job.Name == name }); at >= 0 {
		return f.Run.Run.Jobs[at], true
	}
	return project.Job{}, false
}

func sortedKeys[V any](m map[string]V) []string {
	return slices.Sorted(maps.Keys(m))
}

func names(candidates []Candidate) []string {
	found := make([]string, 0, len(candidates))
	for _, c := range candidates {
		found = append(found, c.Name)
	}
	return found
}

// ----------------------------------------------------------------------------
// The problems

// The codes of a request's problems: a job, a port, a profile or a link naming nothing run init knows; a port spelled wrong or that two jobs declare; a space with a name and no command to make it; a service built here asked shared. Then the problems a surface shows in their part to check, which never refuse the request: a command that binds its port as a literal, ignoring the variable wtm gives it (CodePortIgnored, with the command that reads it when run init can write one), and an .env file whose linked keys a new worktree never gets, config.toml not copying it (CodeEnvNotCopied).
const (
	CodeUnknownJob     = Name + ".unknown_job"
	CodePortInvalid    = Name + ".port_invalid"
	CodePortTwice      = Name + ".port_twice"
	CodeSpaceNoCreate  = Name + ".space_no_create"
	CodeSharedBuilt    = Name + ".shared_built"
	CodeUnknownLink    = Name + ".unknown_link"
	CodeUnknownProfile = Name + ".unknown_profile"
	CodePortIgnored    = Name + ".port_variable_ignored"
	CodeEnvNotCopied   = Name + ".env_file_not_copied"
)

// toCheck reports whether a problem is one to check, which never refuses the request: a link given that holds no port any more is left out.
func toCheck(problem domain.FieldError) bool {
	return problem.Code == CodePortIgnored || problem.Code == CodeEnvNotCopied || problem.Code == CodeUnknownLink
}

func (d Decision) problems(f Facts) []domain.FieldError {
	var problems []domain.FieldError
	unknown := func(at, value string) domain.FieldError {
		return domain.FieldError{Path: at, Code: CodeUnknownJob, Template: "{value} is no job run init can write", Params: domain.Params{"value": value}}
	}
	for i, name := range d.Picked {
		if _, ok := d.candidate(name); !ok {
			problems = append(problems, unknown(PathJobs+"["+strconv.Itoa(i)+"]", name))
		}
	}
	for i, name := range d.given.Shared {
		if c, _ := d.candidate(name); c.Build {
			problems = append(problems, domain.FieldError{Path: PathShared + "[" + strconv.Itoa(i) + "]", Code: CodeSharedBuilt, Template: "{value} is built from this worktree's code: one copy cannot serve every worktree", Params: domain.Params{"value": name}})
		}
	}
	for _, service := range d.Shared {
		if space := d.Spaces[service]; space.Name != "" && strings.TrimSpace(space.Create) == "" {
			problems = append(problems, domain.FieldError{Path: PathSpaces, Code: CodeSpaceNoCreate, Template: "{service} gets a space with no command to make it: give " + "{service}.create", Params: domain.Params{"service": service}})
		}
	}
	problems = append(problems, d.portProblems()...)
	for i, link := range d.Links {
		if link.Job == "" {
			problems = append(problems, domain.FieldError{Path: PathLinks + "[" + strconv.Itoa(i) + "]", Code: CodeUnknownLink, Template: "{value} holds no port of a job", Params: domain.Params{"value": link.Spelled()}})
		}
	}
	for i, profile := range d.Profiles {
		for _, job := range profile.Jobs {
			if _, declared := f.job(job); !slices.Contains(d.Picked, job) && !declared && job != runnerName {
				problems = append(problems, unknown(PathProfiles+"["+strconv.Itoa(i)+"]", job))
			}
		}
	}
	if d.DefaultProfile != "" && !slices.Contains(profileNames(d.Profiles), d.DefaultProfile) {
		problems = append(problems, domain.FieldError{Path: PathDefaultProfile, Code: CodeUnknownProfile, Template: "{value} is no profile", Params: domain.Params{"value": d.DefaultProfile}})
	}

	for i, port := range d.Ports {
		if rewrite, ok := d.Rewrites[port.Job]; ok && d.Commands[port.Job] == "" {
			c, _ := d.candidate(port.Job)
			problems = append(problems, domain.FieldError{Path: PathPorts + "[" + strconv.Itoa(i) + "]", Code: CodePortIgnored, Template: "{job} binds {port} whatever {variable} says", Params: domain.Params{"job": port.Job, "variable": port.Variable, "port": strconv.Itoa(port.Base), "before": c.Cmd, "after": rewrite}})
		}
	}
	for i, file := range d.Uncopied {
		if !slices.Contains(d.EnvCopy, file) && slices.ContainsFunc(d.Links, func(l Link) bool { return l.File == file }) {
			problems = append(problems, domain.FieldError{Path: PathEnvCopy + "[" + strconv.Itoa(i) + "]", Code: CodeEnvNotCopied, Template: "{file} is not copied into a new worktree: its keys keep main's values there", Params: domain.Params{"file": file}})
		}
	}
	return problems
}

// toCheck are the problems to check as the plan's warnings, each with its fix in its params.
func (d Decision) toCheck(f Facts) []domain.Warning {
	var warnings []domain.Warning
	for _, problem := range d.problems(f) {
		if !toCheck(problem) {
			continue
		}
		message := problem.Template
		for key, value := range problem.Params {
			message = strings.ReplaceAll(message, "{"+key+"}", value)
		}
		if after := problem.Params["after"]; after != "" {
			message += ": run it as " + after + " (--command)"
		}
		if problem.Code == CodeEnvNotCopied {
			message += " (--env-copy)"
		}
		warnings = append(warnings, domain.Warning{Code: problem.Code, Message: message, Params: problem.Params})
	}
	return warnings
}

func (d Decision) portProblems() []domain.FieldError {
	var problems []domain.FieldError
	seen := map[int]string{}
	for i, port := range d.Ports {
		at := PathPorts + "[" + strconv.Itoa(i) + "]"
		switch _, ok := d.candidate(port.Job); {
		case !ok:
			problems = append(problems, domain.FieldError{Path: at, Code: CodeUnknownJob, Template: "{value} is no job run init can write", Params: domain.Params{"value": port.Job}})
		case port.Variable == "":
		case envName(port.Variable) != port.Variable || port.Base < 1 || port.Base > 65535:
			problems = append(problems, domain.FieldError{Path: at, Code: CodePortInvalid, Template: "{value} is no port: write job.VARIABLE=base, the base from 1 to 65535", Params: domain.Params{"value": port.Spelled()}})
		case seen[port.Base] != "" && seen[port.Base] != port.Job:
			problems = append(problems, domain.FieldError{Path: at, Code: CodePortTwice, Template: "{value} is the base of {other} too: both would bind it in every worktree", Params: domain.Params{"value": strconv.Itoa(port.Base), "other": seen[port.Base]}})
		default:
			seen[port.Base] = port.Job
		}
	}
	return problems
}

// ----------------------------------------------------------------------------
// Decide

// Decide turns the facts and the settled request into the plan: run.toml written whole from the decision, docker-compose.yml's edits when it is rewritten, config.toml's .env files added, what each file gains, the detection and the preview of a worktree.
func Decide(f Facts, d Decision) Plan {
	plan := Plan{Run: d.run(f), Detection: f.detection(d), replaces: f.Run.Digest}
	if edits := d.composeEdits(f); d.PatchCompose && len(edits) > 0 {
		plan.Compose = &ComposeChange{File: f.Compose.File, Edits: edits}
		plan.compose = &project.ComposeRewrite{File: f.Compose.File, Replaces: f.Compose.Digest}
		for _, edit := range edits {
			plan.compose.Edits = append(plan.compose.Edits, project.ComposeEdit{At: edit.At, New: edit.After})
		}
	}
	if len(d.EnvCopy) > 0 && f.Config != nil {
		plan.Config = &ConfigChange{Added: d.EnvCopy}
		config := f.Config.Config
		config.Env.Files = slices.Clone(config.Env.Files)
		for _, target := range d.EnvCopy {
			config.Env.Files = append(config.Env.Files, f.envFile(target))
		}
		if config.Env.Strategy == "" {
			config.Env.Strategy = project.DefaultEnvStrategy
		}
		plan.config = &project.Rewrite{Config: config, Replaces: f.Config.Digest}
	}
	plan.Parts = d.parts()
	plan.Files = plan.written(f)
	plan.Preview = f.preview(d, plan.Run)
	plan.Recap = plan.recap(d)
	plan.Empty = len(plan.Files) == 0
	return plan
}

// envFile is the .env file of target as config.toml or the repository know it.
func (f Facts) envFile(target string) project.EnvFile {
	if at := slices.IndexFunc(f.EnvFiles, func(file project.EnvFile) bool { return file.Target == target }); at >= 0 {
		file := f.EnvFiles[at]
		file.Local = file.Local && file.Template == ""
		return file
	}
	return project.EnvFile{Target: target}
}

// run is run.toml as the decision writes it: its settings kept, the jobs the request picked in the candidates' order with their ports, address, scope, space and data, those no candidate names kept as they are (run job add's), the profiles and the links.
func (d Decision) run(f Facts) project.Run {
	run := f.Run.Run
	run.Declared, run.Addressing = false, d.Addressing
	var jobs []project.Job
	for _, name := range d.Picked {
		c, ok := d.candidate(name)
		if !ok {
			continue
		}
		jobs = append(jobs, d.job(f, c))
	}
	if d.Runner == Through && d.Runnable != nil {
		runner := d.job(f, *d.Runnable)
		runner.Runs = nil
		for _, name := range d.Picked {
			if d.startedByRunner(name) {
				runner.Runs = append(runner.Runs, name)
			}
		}
		jobs = append(jobs, runner)
	}
	for _, job := range f.Run.Run.Jobs {
		if _, candidate := d.candidate(job.Name); !candidate {
			jobs = append(jobs, job)
		}
	}
	run.Jobs, run.Profiles = jobs, slices.Clone(d.Profiles)

	written := func(name string) bool {
		return slices.ContainsFunc(jobs, func(job project.Job) bool { return job.Name == name })
	}
	run.EnvPorts, run.EnvValues = nil, nil
	for _, link := range f.Run.Run.EnvPorts {
		if _, candidate := d.candidate(link.Job); !candidate && written(link.Job) {
			run.EnvPorts = append(run.EnvPorts, link)
		}
	}
	for _, link := range f.Run.Run.EnvValues {
		if _, candidate := d.candidate(link.Job); !candidate && written(link.Job) {
			run.EnvValues = append(run.EnvValues, link)
		}
	}
	for _, link := range d.Links {
		if !written(link.Job) || link.Port == "" {
			continue
		}
		if link.Kind == LinkData {
			run.EnvValues = append(run.EnvValues, project.EnvValueLink{File: link.File, Key: link.Key, Job: link.Job, Value: link.Template})
			continue
		}
		run.EnvPorts = append(run.EnvPorts, project.EnvPortLink{File: link.File, Key: link.Key, Job: link.Job, Port: link.Port})
	}
	return run
}

// job is the job of c as run.toml writes it: run.toml's own when it declares it, what the decision settles over it.
func (d Decision) job(f Facts, c Candidate) project.Job {
	job, _ := f.job(c.Name)
	job.Name, job.Kind, job.Stop = c.Name, project.Service, c.Stop
	job.Cmd = cmp.Or(d.Commands[c.Name], c.Cmd)
	job.Cwd = ""
	if c.Place != "" && c.Place != "." {
		job.Cwd = c.Place
	}
	if slices.Contains(d.Tasks, c.Name) {
		job.Kind = project.Task
	}

	job.Ports, job.BindsNoPort, job.URL = nil, false, nil
	for _, port := range portsOf(d.Ports, c.Name) {
		if port.Variable == "" {
			job.BindsNoPort = job.Kind == project.Service && c.Name != runnerName
			continue
		}
		if job.Ports == nil {
			job.Ports = map[string]int{}
		}
		job.Ports[port.Variable] = port.Base
	}
	if port, ok := d.firstPort(c.Name); ok && slices.Contains(d.URLs, c.Name) {
		host := ""
		if previous, declared := f.job(c.Name); declared && previous.URL != nil {
			host = previous.URL.Host
		}
		job.URL = &project.URL{Port: port.Variable, Host: host}
	}

	job.Scope, job.Namespace = project.PerWorktree, nil
	if slices.Contains(d.Shared, c.Name) {
		job.Scope = project.Shared
		if space := d.Spaces[c.Name]; space.Name != "" {
			job.Namespace = &space
		}
	}
	job.Touches = nil
	if slices.Contains(d.Touches, c.Name) {
		for _, service := range d.Shared {
			if d.Spaces[service].Name != "" {
				job.Touches = append(job.Touches, service)
			}
		}
	}
	return job
}

// A ComposeEdit is one value of docker-compose.yml the rewrite changes: what it names (a service's port, a container's or a volume's name), where, as written and as written after.
type ComposeEdit struct {
	Subject string              `json:"subject"`
	What    string              `json:"what"`
	At      project.ComposeSpan `json:"at"`
	Before  string              `json:"before"`
	After   string              `json:"after"`
}

// composeEdits are docker-compose.yml's values the rewrite changes: each literal host port of a service picked, to read the variable its port is declared under with the literal as its default; each name a service kept per worktree pins (container_name), a volume's or a network's, fronted with the compose project, so two worktrees never meet on it.
func (d Decision) composeEdits(f Facts) []ComposeEdit {
	var edits []ComposeEdit
	taken := map[string]bool{}
	for _, read := range f.Compose.Reads {
		taken[read] = true
	}
	for _, service := range f.Compose.Services {
		if !slices.Contains(d.Picked, service.Name) || slices.Contains(d.Tasks, service.Name) {
			continue
		}
		for _, port := range f.composePorts(composePortsParams{Service: service.Name, Patch: true, Taken: maps.Clone(taken)}) {
			for _, published := range service.Ports {
				if published.Unreadable != "" || published.Host != strconv.Itoa(port.Base) || port.Variable == "" {
					continue
				}
				quote := ""
				if strings.HasPrefix(published.At.Text, `"`) || published.IP == "" {
					quote = `"`
				}
				after := "${" + port.Variable + ":-" + published.Host + "}"
				if published.At.Text != published.Host {
					after = quote + strings.TrimPrefix(published.IP+":", ":") + after + ":" + published.Container + quote
				}
				edits = append(edits, ComposeEdit{Subject: service.Name, What: "port", At: published.At, Before: published.At.Text, After: after})
			}
		}
	}
	for _, name := range f.Compose.Names {
		if name.Unreadable != "" || strings.Contains(name.Value, "$") || name.Kind == project.ComposeContainer && slices.Contains(d.Shared, name.Owner) {
			continue
		}
		project := worktree.Slug(f.Project)
		suffix := strings.TrimPrefix(strings.TrimPrefix(name.Value, project+"-"), f.Project+"-")
		after := `"${COMPOSE_PROJECT_NAME:-` + project + `}-` + suffix + `"`
		if suffix == project || suffix == f.Project {
			after = `"${COMPOSE_PROJECT_NAME:-` + project + `}"`
		}
		edits = append(edits, ComposeEdit{Subject: name.Owner, What: string(name.Kind), At: name.At, Before: name.At.Text, After: after})
	}
	return edits
}

// ----------------------------------------------------------------------------
// The plan

// A Plan is what Apply will write, as the dry run and the JSON envelope show it and the fingerprint covers it: run.toml whole (which the envelope leaves to the detection and the preview), docker-compose.yml's edits, config.toml's .env files added, and the files written with what each gains; then what the repository holds (Detection) and what the worktree previewed gets (Preview), which a surface shows beside the questions; then the recap.
type Plan struct {
	Run       project.Run    `json:"-"`
	Compose   *ComposeChange `json:"compose,omitempty"`
	Config    *ConfigChange  `json:"config,omitempty"`
	Files     []File         `json:"files"`
	Detection Detection      `json:"detection"`
	Parts     Parts          `json:"parts"`
	Preview   Preview        `json:"preview"`
	domain.Recapped
	compose  *project.ComposeRewrite
	config   *project.Rewrite
	replaces string
}

// ComposeChange is docker-compose.yml rewritten: its path and its edits.
type ComposeChange struct {
	File  string        `json:"file"`
	Edits []ComposeEdit `json:"edits"`
}

// ConfigChange is config.toml's .env files added.
type ConfigChange struct {
	Added []string `json:"added"`
}

// A File is a file the apply writes: its path (relative to the repository, or to wtm's state folder for run.toml and config.toml), whether it lives in the repository, tracked by git, rather than in wtm's state, and whether it is new.
type File struct {
	Path    string `json:"path"`
	Tracked bool   `json:"tracked"`
	New     bool   `json:"new"`
}

// written are the files the plan writes: run.toml unless it holds already what the plan writes, docker-compose.yml when it is rewritten, config.toml when it copies a file more.
func (p Plan) written(f Facts) []File {
	var files []File
	onDisk := f.Run.Run
	onDisk.Declared = false
	if !f.declared() || !sameRun(onDisk, p.Run) {
		files = append(files, File{Path: runFile, New: !f.declared()})
	}
	if p.Compose != nil {
		files = append(files, File{Path: p.Compose.File, Tracked: true})
	}
	if p.Config != nil {
		files = append(files, File{Path: "config.toml"})
	}
	return files
}

// sameRun reports whether two run configs write the same run.toml.
func sameRun(a, b project.Run) bool {
	normal := func(run project.Run) project.Run {
		for i := range run.Jobs {
			if len(run.Jobs[i].Ports) == 0 {
				run.Jobs[i].Ports = nil
			}
		}
		return run
	}
	return equalJSON(normal(a), normal(b))
}

// Locks are the locks the plan needs: the repository's, shared; run init changes no worktree.
func (p Plan) Locks() []domain.Lock {
	return domain.WorktreeLocks(nil)
}

// Locks are the locks taken before planning: the repository's, shared.
func (r Request) Locks() []domain.Lock {
	return domain.WorktreeLocks(nil)
}

// Fingerprinted is what the plan's fingerprint covers: what it writes and the files it writes over, as read; the detection, the preview and the recap left out.
func (p Plan) Fingerprinted() any {
	type written struct {
		Run      project.Run             `json:"run"`
		Compose  *project.ComposeRewrite `json:"compose"`
		Config   *project.Rewrite        `json:"config"`
		Replaces string                  `json:"replaces"`
	}
	return written{Run: p.Run, Compose: p.compose, Config: p.config, Replaces: p.replaces}
}

// equalJSON reports whether a and b encode the same.
func equalJSON(a, b any) bool {
	left, errA := json.Marshal(a)
	right, errB := json.Marshal(b)
	return errA == nil && errB == nil && string(left) == string(right)
}

// ----------------------------------------------------------------------------
// The parts

// Parts are the rows of the parts a surface lays out as a table, each as the decision settled it, with the answer that spells it (Spelled), which a surface sends back unchanged, never builds: the ports, the commands changed and those proposed, each shared service's space, the .env keys that can follow a job and whether each does, and the profiles, those proposed besides.
type Parts struct {
	Ports    []PortRow    `json:"ports"`
	Commands []CommandRow `json:"commands"`
	Spaces   []SpaceRow   `json:"spaces"`
	Links    []LinkRow    `json:"links"`
	Profiles []ProfileRow `json:"profiles"`
}

// A PortRow is a job's port, spelled.
type PortRow struct {
	Port
	Spelled string `json:"spelled"`
}

// A CommandRow is a job's command: as it runs, the one run init proposes so it reads its port's variable (none when it proposes none), and whether the request changed it; Spelled answers it with the proposal.
type CommandRow struct {
	Job      string `json:"job"`
	Cmd      string `json:"cmd"`
	Proposed string `json:"proposed,omitempty"`
	Changed  bool   `json:"changed"`
	Spelled  string `json:"spelled,omitempty"`
}

// A SpaceRow is what each worktree gets of a shared service, and the answers that spell it.
type SpaceRow struct {
	Service string   `json:"service"`
	Name    string   `json:"name"`
	Create  string   `json:"create"`
	Remove  string   `json:"remove"`
	Spelled []string `json:"spelled"`
}

// A LinkRow is an .env key that can follow a job, whether it does, and its answer.
type LinkRow struct {
	Link
	Linked  bool   `json:"linked"`
	Spelled string `json:"spelled"`
}

// A ProfileRow is a profile: its jobs, whether it is the default, whether run init proposes it rather than run.toml or the request holding it, and its answer.
type ProfileRow struct {
	Name     string   `json:"name"`
	Jobs     []string `json:"jobs"`
	Default  bool     `json:"default"`
	Proposed bool     `json:"proposed"`
	Spelled  string   `json:"spelled"`
}

func (d Decision) parts() Parts {
	parts := Parts{Ports: []PortRow{}, Commands: []CommandRow{}, Spaces: []SpaceRow{}, Links: []LinkRow{}, Profiles: []ProfileRow{}}
	for _, port := range d.Ports {
		parts.Ports = append(parts.Ports, PortRow{Port: port, Spelled: port.Spelled()})
	}
	for _, name := range d.Picked {
		c, ok := d.candidate(name)
		_, changed := d.Commands[name]
		if !ok || d.Rewrites[name] == "" && !changed {
			continue
		}
		row := CommandRow{Job: name, Cmd: cmp.Or(d.Commands[name], c.Cmd), Proposed: d.Rewrites[name], Changed: changed}
		if row.Proposed != "" {
			row.Spelled = name + "=" + row.Proposed
		}
		parts.Commands = append(parts.Commands, row)
	}
	for _, service := range d.Shared {
		space := d.Spaces[service]
		parts.Spaces = append(parts.Spaces, SpaceRow{Service: service, Name: space.Name, Create: space.Create, Remove: space.Remove, Spelled: spelledSpaces([]string{service}, d.Spaces)})
	}
	for _, link := range d.Linkable {
		linked := slices.ContainsFunc(d.Links, func(l Link) bool { return l.Spelled() == link.Spelled() })
		parts.Links = append(parts.Links, LinkRow{Link: link, Linked: linked, Spelled: link.Spelled()})
	}
	profiles := slices.Clone(d.Profiles)
	for _, proposed := range d.ProposedProfiles {
		if !slices.ContainsFunc(profiles, func(p project.Profile) bool { return p.Name == proposed.Name }) {
			profiles = append(profiles, proposed)
		}
	}
	for i, profile := range profiles {
		proposed := i >= len(d.Profiles) || d.Origins[PathProfiles] == domain.OriginDetected
		parts.Profiles = append(parts.Profiles, ProfileRow{Name: profile.Name, Jobs: profile.Jobs, Default: profile.Name == d.DefaultProfile && i < len(d.Profiles), Proposed: proposed, Spelled: profile.Name + "=" + strings.Join(profile.Jobs, ",")})
	}
	return parts
}

// ----------------------------------------------------------------------------
// The detection

// Detection is what run init read of the repository, for a surface to show beside its questions: the package manager and the file that says so, docker-compose.yml's services with the ports they publish as written, each place with its scripts, the jobs run init may write and the root script that starts the apps, and each .env file with whether config.toml copies it and its keys, each with the job it follows.
type Detection struct {
	Manager     string          `json:"manager"`
	ManagerFrom string          `json:"manager_from"`
	Compose     string          `json:"compose"`
	Services    []Service       `json:"services"`
	Places      []project.Place `json:"places"`
	Candidates  []Candidate     `json:"candidates"`
	Runner      *Candidate      `json:"runner,omitempty"`
	EnvFiles    []EnvFile       `json:"env_files"`
}

// A Service is a service of docker-compose.yml: its name, its image, and its ports as written, host:container.
type Service struct {
	Name  string   `json:"name"`
	Image string   `json:"image"`
	Ports []string `json:"ports"`
}

// An EnvFile is an .env file a worktree may hold: its path, whether config.toml copies it into a new worktree, and its keys.
type EnvFile struct {
	File   string   `json:"file"`
	Copied bool     `json:"copied"`
	Keys   []EnvKey `json:"keys"`
}

// An EnvKey is a key of an .env file, its value as written, and the job and kind of link it follows (none when it follows none).
type EnvKey struct {
	Key   string   `json:"key"`
	Value string   `json:"value"`
	Job   string   `json:"job,omitempty"`
	Kind  LinkKind `json:"kind,omitempty"`
}

func (f Facts) detection(d Decision) Detection {
	detection := Detection{Manager: f.Workspace.Manager, ManagerFrom: f.Workspace.ManagerFrom, Compose: f.Compose.File, Places: f.Workspace.Places, Candidates: d.Candidates, Runner: d.Runnable, Services: []Service{}, EnvFiles: []EnvFile{}}
	for _, service := range f.Compose.Services {
		found := Service{Name: service.Name, Image: service.Image, Ports: []string{}}
		for _, port := range service.Ports {
			found.Ports = append(found.Ports, strings.TrimPrefix(port.IP+":", ":")+port.Host+":"+port.Container)
		}
		detection.Services = append(detection.Services, found)
	}
	for _, file := range f.EnvFiles {
		source := file.Target
		if _, ok := f.EnvLines[source]; !ok {
			source = file.Template
		}
		found := EnvFile{File: file.Target, Copied: f.copied(file.Target), Keys: []EnvKey{}}
		for _, line := range f.EnvLines[source] {
			if line.Kind != dotenv.Pair {
				continue
			}
			key := EnvKey{Key: line.Key, Value: line.Value}
			if at := slices.IndexFunc(d.Links, func(l Link) bool { return l.File == file.Target && l.Key == line.Key }); at >= 0 {
				key.Job, key.Kind = d.Links[at].Job, d.Links[at].Kind
			}
			found.Keys = append(found.Keys, key)
		}
		detection.EnvFiles = append(detection.EnvFiles, found)
	}
	return detection
}

// ----------------------------------------------------------------------------
// The preview

// Preview is what the worktree previewed gets from the plan: its branch, ordinal and how far its ports sit above main's; the services and tasks it runs and the shared services it joins; the space each shared service gives it; each port with its base; the address of each job answering under its name; the keys of each .env file written in it; and what run up starts there.
type Preview struct {
	Worktree  string           `json:"worktree"`
	Ordinal   int              `json:"ordinal"`
	Offset    int              `json:"offset"`
	Services  []string         `json:"services"`
	Tasks     []string         `json:"tasks"`
	Joins     []string         `json:"joins"`
	Data      []PreviewData    `json:"data"`
	Ports     []PreviewPort    `json:"ports"`
	Addresses []PreviewAddress `json:"addresses"`
	Env       []PreviewEnv     `json:"env"`
	Up        project.Profile  `json:"up"`
}

// PreviewData is the space a shared service gives the worktree: its name there, from its template.
type PreviewData struct {
	Service  string `json:"service"`
	Name     string `json:"name"`
	Template string `json:"template"`
}

// PreviewPort is a port of the worktree: the job, its variable, its base and the port it binds there, the base on a shared job.
type PreviewPort struct {
	Job      string `json:"job"`
	Variable string `json:"variable"`
	Base     int    `json:"base"`
	Port     int    `json:"port"`
	Shared   bool   `json:"shared"`
}

// PreviewAddress is the address a job answers under in the worktree.
type PreviewAddress struct {
	Job string `json:"job"`
	URL string `json:"url"`
}

// PreviewEnv is an .env file written in the worktree and the keys wtm writes in it.
type PreviewEnv struct {
	File string   `json:"file"`
	Keys []string `json:"keys"`
}

func (f Facts) preview(d Decision, run project.Run) Preview {
	preview := Preview{Worktree: d.Preview, Services: []string{}, Tasks: []string{}, Joins: []string{}, Data: []PreviewData{}, Ports: []PreviewPort{}, Addresses: []PreviewAddress{}, Env: []PreviewEnv{}}
	slug := worktree.Slug(d.Preview)
	if at := slices.IndexFunc(f.Worktrees, func(w Previewable) bool { return w.Branch == d.Preview }); at >= 0 && !f.Worktrees[at].Main {
		preview.Ordinal = f.Worktrees[at].Ordinal
		preview.Offset = preview.Ordinal * env.OffsetBlock(run)
	}
	offset := preview.Offset

	for _, job := range run.Jobs {
		switch {
		case job.Scope == project.Shared:
			preview.Joins = append(preview.Joins, job.Name)
		case job.Kind == project.Task:
			preview.Tasks = append(preview.Tasks, job.Name)
		default:
			preview.Services = append(preview.Services, job.Name)
		}
		if job.Namespace != nil {
			name := strings.NewReplacer("{worktree}", slug, "{ordinal}", strconv.Itoa(preview.Ordinal)).Replace(job.Namespace.Name)
			preview.Data = append(preview.Data, PreviewData{Service: job.Name, Name: name, Template: job.Namespace.Name})
		}
		for name, port := range env.PortsAt(job, &offset) {
			preview.Ports = append(preview.Ports, PreviewPort{Job: job.Name, Variable: name, Base: job.Ports[name], Port: port, Shared: job.Scope == project.Shared})
		}
		if job.URL != nil {
			preview.Addresses = append(preview.Addresses, PreviewAddress{Job: job.Name, URL: f.address(addressParams{Job: job, Slug: slug, Offset: offset, Addressing: run.Addressing})})
		}
	}
	slices.SortStableFunc(preview.Ports, func(a, b PreviewPort) int { return cmp.Compare(a.Job+a.Variable, b.Job+b.Variable) })

	keys := map[string][]string{}
	var files []string
	add := func(file, key string) {
		if keys[file] == nil {
			files = append(files, file)
		}
		keys[file] = append(keys[file], key)
	}
	for _, link := range run.EnvPorts {
		add(link.File, link.Key)
	}
	for _, link := range run.EnvValues {
		add(link.File, link.Key)
	}
	for _, file := range files {
		preview.Env = append(preview.Env, PreviewEnv{File: file, Keys: keys[file]})
	}

	if at := slices.IndexFunc(run.Profiles, func(p project.Profile) bool { return p.Default }); at >= 0 {
		preview.Up = run.Profiles[at]
	}
	return preview
}

type addressParams struct {
	Job        project.Job
	Slug       string
	Offset     int
	Addressing project.Addressing
}

// address is the URL a job answers under in the worktree: its name through the proxy, else its port.
func (f Facts) address(params addressParams) string {
	if params.Addressing != project.AddressPorts && f.ProxyPort > 0 {
		host := env.RouteHost(env.RouteHostParams{Job: params.Job, Slug: params.Slug, Project: f.Project})
		if f.ProxyPort == 80 {
			return "http://" + host
		}
		return "http://" + host + ":" + strconv.Itoa(f.ProxyPort)
	}
	port := env.PortsAt(params.Job, &params.Offset)[params.Job.URL.Port]
	return "http://localhost:" + strconv.Itoa(port)
}

// ----------------------------------------------------------------------------
// The recap

// recap is the plan row by row: the jobs, the shared services, the ports, the addresses, the .env keys linked, the profiles and the files written.
func (p Plan) recap(d Decision) []domain.RecapRow {
	var jobs, shared, ports, profiles, files []string
	for _, job := range p.Run.Jobs {
		jobs = append(jobs, job.Name)
		if job.Scope == project.Shared {
			shared = append(shared, job.Name)
		}
		for _, name := range slices.Sorted(maps.Keys(job.Ports)) {
			ports = append(ports, job.Name+" "+name+" "+strconv.Itoa(job.Ports[name]))
		}
	}
	for _, profile := range p.Run.Profiles {
		name := profile.Name
		if profile.Default {
			name += " (default)"
		}
		profiles = append(profiles, name)
	}
	for _, file := range p.Files {
		files = append(files, file.Path)
	}
	rows := []domain.RecapRow{{Key: "jobs", Label: "Jobs", Path: PathJobs, Values: domain.RecapValues(jobs)}}
	if len(shared) > 0 {
		rows = append(rows, domain.RecapRow{Key: "shared", Label: "Shared", Path: PathShared, Values: domain.RecapValues(shared)})
	}
	if len(ports) > 0 {
		rows = append(rows, domain.RecapRow{Key: "ports", Label: "Ports", Path: PathPorts, Values: domain.RecapValues(ports)})
	}
	rows = append(rows, domain.RecapRow{Key: "addressing", Label: "Addresses", Path: PathAddressing, Values: domain.RecapValues([]string{string(d.Addressing)})})
	if n := len(p.Run.EnvPorts) + len(p.Run.EnvValues); n > 0 {
		rows = append(rows, domain.RecapRow{Key: "links", Label: ".env keys", Path: PathLinks, Values: domain.RecapValues([]string{strconv.Itoa(n) + " linked"})})
	}
	if len(profiles) > 0 {
		rows = append(rows, domain.RecapRow{Key: "profiles", Label: "Profiles", Path: PathProfiles, Values: domain.RecapValues(profiles)})
	}
	if len(files) > 0 {
		rows = append(rows, domain.RecapRow{Key: "writes", Label: "Writes", Values: domain.RecapValues(files)})
	}
	return rows
}

// ----------------------------------------------------------------------------
// The outcome

// CodeUp is the follow-up of a run init: start the default profile.
const CodeUp = Name + ".up"

// next are the follow-ups of a run init that wrote a profile: run up.
func (p Plan) next() []domain.FollowUp {
	if len(p.Run.Profiles) == 0 && len(p.Run.Jobs) == 0 {
		return nil
	}
	return []domain.FollowUp{{Command: domain.CommandRunUp, Code: CodeUp, Message: "start the jobs"}}
}

// CodeComposeUnreadable warns of a docker compose file that does not parse: its services are left out.
const CodeComposeUnreadable = Name + ".compose_unreadable"

// warnings are what the facts warn of: a docker compose file that does not parse.
func (f Facts) warnings() []domain.Warning {
	if f.Compose.Err == "" {
		return nil
	}
	return []domain.Warning{{Code: CodeComposeUnreadable, Message: f.Compose.File + " does not parse, its services are left out: " + f.Compose.Err, Params: domain.Params{"file": f.Compose.File}}}
}
