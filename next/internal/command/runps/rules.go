package runps

import (
	"cmp"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

// ----------------------------------------------------------------------------
// Facts

// Facts are what run ps reads: what the daemon was asked (its jobs, or why it could not tell), each repository its jobs run for, a dir whose repository could not be read with why, and the last line of each job a group's profile declares that the daemon does not list, read from its log (nil when it holds none).
type Facts struct {
	repository.Asked
	Projects   []Project
	Unreadable map[string]error // by the directory of a job whose repository could not be read
	Logged     map[LogKey]*run.Line
}

// A Project is one repository as run ps reads it: its config, run.toml and worktrees, the meta record of each worktree by branch, and where wtm keeps its state.
type Project struct {
	worktree.Repository
	Records  map[string]worktree.Meta
	StateDir string
}

// A LogKey names one job's log: the worktree it runs in, and the job.
type LogKey struct {
	Path string
	Job  string
}

// RepoOf is the main checkout of the repository job runs for, as the daemon names it: the repository its start named, else the main checkout a shared job runs in, else "" (the daemon does not say).
func RepoOf(job run.Job) string {
	return cmp.Or(job.Repo, job.SharedDir)
}

// Unlisted are the jobs of the groups of projects that the daemon does not list in them: each job of the profile a worktree's jobs were started under, neither shared nor started by a runner of the worktree; their logs say whether they ran.
func Unlisted(projects []Project, jobs []run.Job) map[LogKey]run.LogFile {
	unlisted := map[LogKey]run.LogFile{}
	for _, p := range projects {
		for _, g := range groupsOf(p, jobs) {
			for _, job := range p.declaredIdle(g.path, jobs) {
				unlisted[LogKey{Path: g.path, Job: job.Name}] = run.LogFile{Dir: run.LogDir(p.StateDir, p.branchAt(g.path)), Job: job.Name}
			}
		}
	}
	return unlisted
}

// ----------------------------------------------------------------------------
// The detail

// The codes of what run ps could not read.
const (
	// CodeRepoUnreadable warns of jobs whose repository could not be read: they are listed without what run.toml says of them.
	CodeRepoUnreadable = Name + ".repo_unreadable"
)

// A Detail is one group of jobs as the JSON envelope reports it: the jobs one worktree runs for itself, or a repository's shared services (Shared), with the repository's main checkout, the worktree's branch (none for the shared services or a worktree on no branch) and path (the main checkout for the shared services), the profile its jobs were started under, how many of its jobs stand in each state, and its jobs, those to see first first.
type Detail struct {
	Repository string `json:"repository"`
	Shared     bool   `json:"shared"`
	Branch     string `json:"branch"`
	Path       string `json:"path"`
	Profile    string `json:"profile"`
	Counts     Counts `json:"counts"`
	Jobs       []Job  `json:"jobs"`
}

// Counts are how many jobs of a group stand in each state, and, for the shared services, how many worktrees hold one.
type Counts struct {
	Running    int `json:"running"`
	Starting   int `json:"starting"`
	Crashed    int `json:"crashed"`
	Exited     int `json:"exited"`
	Stopped    int `json:"stopped"`
	NotStarted int `json:"not_started"`
	Holders    int `json:"holders,omitempty"`
}

// A Job is one job of a group: its name and kind, its state, what of it the daemon holds (none for one it does not list), its url and where it answers on this machine (host:port), when it started, when it ended and how long it ran (milliseconds), the code it ended with (a crash's or a task's), the last line it printed, the folder it runs in (run.toml's cwd) and its command, the file its lines are kept in; the services a runner starts, and the worktrees holding a shared service.
type Job struct {
	Name       string          `json:"name"`
	Kind       project.JobKind `json:"kind,omitempty"`
	State      run.State       `json:"state"`
	Hold       run.Hold        `json:"hold,omitempty"`
	URL        string          `json:"url,omitempty"`
	Address    string          `json:"address,omitempty"`
	StartedAt  *time.Time      `json:"started_at,omitempty"`
	EndedAt    *time.Time      `json:"ended_at,omitempty"`
	DurationMS *int64          `json:"duration_ms,omitempty"`
	ExitCode   *int            `json:"exit_code,omitempty"`
	LastLine   *Line           `json:"last_line,omitempty"`
	Dir        string          `json:"dir,omitempty"`
	Command    string          `json:"command,omitempty"`
	Log        string          `json:"log,omitempty"`
	Services   []Service       `json:"services,omitempty"`
	Holders    []Holder        `json:"holders,omitempty"`
}

// A Line is a line a job printed: when, and its text.
type Line struct {
	At   time.Time `json:"at,omitzero"`
	Text string    `json:"text"`
}

// A Service is a job a runner starts itself (run.toml's runs): its name and where it answers on this machine, none when it binds no port or the worktree's ports cannot be told.
type Service struct {
	Name    string `json:"name"`
	Address string `json:"address,omitempty"`
}

// A Holder is a worktree holding a shared service: its branch (none on no branch) and path, the namespace the service made for it (none when it makes none), and since when it holds it.
type Holder struct {
	Branch    string     `json:"branch"`
	Path      string     `json:"path"`
	Namespace string     `json:"namespace,omitempty"`
	Since     *time.Time `json:"since,omitempty"`
}

// ----------------------------------------------------------------------------
// List

// List reports the jobs of each repository read, a group per worktree running jobs for itself, those with a crash first, then the main one, then the others in git's order; then the repository's shared services, each with the worktrees holding it. A repository that could not be read is warned, its jobs grouped all the same.
func List(f Facts) ([]domain.Unit[Detail], []domain.Warning) {
	var items []domain.Unit[Detail]
	for _, p := range f.Projects {
		var groups []domain.Unit[Detail]
		for _, g := range groupsOf(p, f.Jobs) {
			groups = append(groups, unit(p.group(g, f)))
		}
		slices.SortStableFunc(groups, func(a, b domain.Unit[Detail]) int {
			return cmp.Compare(rank(a.Detail.Counts.Crashed > 0), rank(b.Detail.Counts.Crashed > 0))
		})
		items = append(items, groups...)
		if shared := p.shared(f.Jobs); len(shared.Jobs) > 0 {
			items = append(items, unit(shared))
		}
	}

	warnings := f.Warnings(Name)
	for _, dir := range slices.Sorted(maps.Keys(f.Unreadable)) {
		warnings = append(warnings, domain.Warning{Code: CodeRepoUnreadable, Message: "couldn't read the repository of the jobs in " + dir + ", so run.toml says nothing of them: " + f.Unreadable[dir].Error(), Params: domain.Params{"dir": dir}})
	}
	return items, warnings
}

// unit is the group read: its subject the worktree's branch (its path on none), or shared.
func unit(d Detail) domain.Unit[Detail] {
	subject := cmp.Or(d.Branch, d.Path)
	if d.Shared {
		subject = "shared"
	}
	return domain.Unit[Detail]{Subject: subject, Status: domain.StatusDone, Detail: &d}
}

// rank is 0 for true, 1 for false: what sorts first.
func rank(first bool) int {
	if first {
		return 0
	}
	return 1
}

// a group is a worktree of a project running jobs for itself: its path, and whether it is the main one.
type group struct {
	path string
	main bool
}

// groupsOf are the worktrees of p running jobs for themselves, the main one first, then in git's order, then those git no longer lists by path.
func groupsOf(p Project, jobs []run.Job) []group {
	paths := map[string]bool{}
	for _, job := range jobs {
		if run.Own(job) && p.owns(job) {
			paths[job.WorkDir] = true
		}
	}

	var groups []group
	for _, w := range p.Worktrees {
		if paths[w.Path] {
			groups = append(groups, group{path: w.Path, main: w.Main})
			delete(paths, w.Path)
		}
	}
	for _, path := range slices.Sorted(maps.Keys(paths)) {
		groups = append(groups, group{path: path})
	}
	return groups
}

// owns reports whether job runs for p: the repository its start named, else the worktree it runs in.
func (p Project) owns(job run.Job) bool {
	if repo := RepoOf(job); repo != "" {
		return repo == p.MainPath
	}
	return p.worktreeAt(job.WorkDir) != nil || job.WorkDir == p.MainPath
}

// worktreeAt is p's worktree at path, nil when git does not list one there.
func (p Project) worktreeAt(path string) *worktree.Worktree {
	at := slices.IndexFunc(p.Worktrees, func(w worktree.Worktree) bool { return w.Path == path })
	if at < 0 {
		return nil
	}
	return &p.Worktrees[at]
}

// branchAt is the branch of p's worktree at path, "" on none or when git lists none there.
func (p Project) branchAt(path string) string {
	if w := p.worktreeAt(path); w != nil {
		return w.Branch()
	}
	return ""
}

// group is the jobs the worktree of g runs for itself, then those of its profile the daemon does not list there.
func (p Project) group(g group, f Facts) Detail {
	d := Detail{Repository: p.MainPath, Branch: p.branchAt(g.path), Path: g.path, Profile: run.ProfileIn(f.Jobs, g.path), Jobs: []Job{}}
	offset := p.offset(g.path)
	for _, job := range f.Jobs {
		if job.WorkDir == g.path && run.Own(job) && p.owns(job) {
			d.Jobs = append(d.Jobs, p.jobOf(job, offset))
		}
	}
	for _, declared := range p.declaredIdle(g.path, f.Jobs) {
		idle := Job{Name: declared.Name, Kind: declared.Kind, State: run.Unlisted(f.Logged[LogKey{Path: g.path, Job: declared.Name}] != nil), Dir: declared.Cwd, Command: declared.Cmd, Log: p.logOf(g.path, declared.Name)}
		if last := f.Logged[LogKey{Path: g.path, Job: declared.Name}]; last != nil {
			idle.LastLine = &Line{At: last.At, Text: last.Text}
		}
		d.Jobs = append(d.Jobs, idle)
	}

	p.order(d.Jobs)
	d.Counts = countsOf(d.Jobs)
	return d
}

// declaredIdle are the jobs of the profile the worktree at path runs that the daemon does not list there, in run.toml's order: neither a shared service (listed with the shared services) nor a job a runner listed there starts itself.
func (p Project) declaredIdle(path string, jobs []run.Job) []project.Job {
	profile := run.ProfileIn(jobs, path)
	at := slices.IndexFunc(p.Run.Profiles, func(pr project.Profile) bool { return pr.Name == profile })
	if profile == "" || at < 0 || p.RunErr != nil {
		return nil
	}

	started := map[string]bool{}
	for _, job := range jobs {
		if job.WorkDir == path {
			started[job.Name] = true
			for _, child := range p.declared(job.Name).Runs {
				started[child] = true
			}
		}
	}
	var idle []project.Job
	for _, name := range p.Run.Profiles[at].Jobs {
		if declared := p.declared(name); !started[name] && declared.Name != "" && declared.Scope != project.Shared {
			idle = append(idle, declared)
		}
	}
	return idle
}

// declared is the job run.toml declares under name, the zero Job when it declares none.
func (p Project) declared(name string) project.Job {
	at := slices.IndexFunc(p.Run.Jobs, func(job project.Job) bool { return job.Name == name })
	if at < 0 || p.RunErr != nil {
		return project.Job{}
	}
	return p.Run.Jobs[at]
}

// jobOf is job as run ps reports it, the services it starts at the worktree's ports, offset above the main worktree's (nil when that cannot be told).
func (p Project) jobOf(job run.Job, offset *int) Job {
	declared := p.declared(job.Name)
	found := Job{Name: job.Name, Kind: job.Kind, State: job.State, Hold: job.Hold, URL: job.URL, Dir: declared.Cwd, Command: declared.Cmd, Log: p.logOf(cmp.Or(job.SharedDir, job.WorkDir), job.Name)}
	// A runner answers through the services it starts, each at its own address.
	if len(declared.Runs) == 0 {
		found.Address = addressOf(declared, job.Ports)
	}
	if job.State == run.Crashed || job.State == run.Exited {
		found.ExitCode = job.ExitCode
	}
	if !job.StartedAt.IsZero() {
		found.StartedAt = &job.StartedAt
	}
	if !job.EndedAt.IsZero() {
		found.EndedAt = &job.EndedAt
		if found.StartedAt != nil {
			found.DurationMS = new(job.EndedAt.Sub(job.StartedAt).Milliseconds())
		}
	}
	if job.LastLine != nil {
		found.LastLine = &Line{At: job.LastLine.At, Text: job.LastLine.Text}
	}
	for _, name := range declared.Runs {
		child := p.declared(name)
		found.Services = append(found.Services, Service{Name: name, Address: addressOf(child, env.PortsAt(child, offset))})
	}
	return found
}

// logOf is the file the lines of the job called name are kept in when it runs in the worktree at path, "" for a worktree on no branch.
func (p Project) logOf(path, name string) string {
	dir := run.LogDir(p.StateDir, p.branchAt(path))
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, name+".log")
}

// offset is how far the ports of the worktree at path sit above the main worktree's, nil when that cannot be told.
func (p Project) offset(path string) *int {
	w := p.worktreeAt(path)
	if w == nil {
		return nil
	}
	return env.OffsetOf(env.OffsetParams{Worktree: *w, Worktrees: p.Worktrees, Records: p.Records, Run: p.Run})
}

// addressOf is where job answers on this machine, given the ports it binds: localhost at its url's port, else at its one port; "" when it binds none, or several and no url names one.
func addressOf(job project.Job, ports map[string]int) string {
	port := 0
	switch {
	case job.URL != nil:
		port = ports[job.URL.Port]
	case len(ports) == 1:
		for _, only := range ports {
			port = only
		}
	}
	if port == 0 {
		return ""
	}
	return "localhost:" + strconv.Itoa(port)
}

// order sorts jobs to see first first: crashed, then starting, running, exited, stopped and never started; among one state, in run.toml's order, then by name.
func (p Project) order(jobs []Job) {
	at := func(name string) int {
		i := slices.IndexFunc(p.Run.Jobs, func(job project.Job) bool { return job.Name == name })
		if i < 0 {
			return len(p.Run.Jobs)
		}
		return i
	}
	slices.SortStableFunc(jobs, func(a, b Job) int {
		return cmp.Or(cmp.Compare(slices.Index(stateOrder, a.State), slices.Index(stateOrder, b.State)), cmp.Compare(at(a.Name), at(b.Name)), strings.Compare(a.Name, b.Name))
	})
}

// stateOrder is the order a group lists its jobs by their state.
var stateOrder = []run.State{run.Crashed, run.Starting, run.Running, run.Exited, run.Stopped, run.NotStarted}

// countsOf counts jobs by their state.
func countsOf(jobs []Job) Counts {
	var c Counts
	for _, job := range jobs {
		switch job.State {
		case run.Running:
			c.Running++
		case run.Starting:
			c.Starting++
		case run.Crashed:
			c.Crashed++
		case run.Exited:
			c.Exited++
		case run.Stopped:
			c.Stopped++
		case run.NotStarted:
			c.NotStarted++
		}
	}
	return c
}

// shared is p's shared services, each the job its main checkout runs, with the worktrees holding it: the main one first when it holds it, then the others in git's order.
func (p Project) shared(jobs []run.Job) Detail {
	d := Detail{Repository: p.MainPath, Shared: true, Path: p.MainPath, Jobs: []Job{}}
	holders := map[string]bool{}
	for _, job := range jobs {
		if job.Hold == run.HoldClaim || job.SharedDir == "" || !p.owns(job) {
			continue
		}
		found := p.jobOf(job, nil)
		found.Address = addressOf(p.declared(job.Name), job.Ports)
		found.Holders = p.holdersOf(job, jobs)
		for _, h := range found.Holders {
			holders[h.Path] = true
		}
		d.Jobs = append(d.Jobs, found)
	}

	p.order(d.Jobs)
	d.Counts = countsOf(d.Jobs)
	d.Counts.Holders = len(holders)
	return d
}

// holdersOf are the worktrees holding the shared job: its main checkout when it holds it, then each worktree claiming it, in git's order.
func (p Project) holdersOf(job run.Job, jobs []run.Job) []Holder {
	var holders []Holder
	if job.MainHolds {
		holders = append(holders, p.holder(job))
	}
	var claims []run.Job
	for _, claim := range jobs {
		if claim.Hold == run.HoldClaim && claim.Name == job.Name && claim.SharedDir == job.SharedDir {
			claims = append(claims, claim)
		}
	}
	slices.SortStableFunc(claims, func(a, b run.Job) int { return cmp.Compare(p.position(a.WorkDir), p.position(b.WorkDir)) })
	for _, claim := range claims {
		holders = append(holders, p.holder(claim))
	}
	return holders
}

// holder is the worktree holding a shared service through hold.
func (p Project) holder(hold run.Job) Holder {
	h := Holder{Branch: p.branchAt(hold.WorkDir), Path: hold.WorkDir, Namespace: hold.Namespace}
	if !hold.StartedAt.IsZero() {
		h.Since = &hold.StartedAt
	}
	return h
}

// position is where git lists the worktree at path, after them all when it does not.
func (p Project) position(path string) int {
	at := slices.IndexFunc(p.Worktrees, func(w worktree.Worktree) bool { return w.Path == path })
	if at < 0 {
		return len(p.Worktrees)
	}
	return at
}
