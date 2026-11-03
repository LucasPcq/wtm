package runlogs

import (
	"cmp"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

// ----------------------------------------------------------------------------
// Facts

// Facts are what run logs reads from, as Observe read them: the project's config, its run.toml, the repository's snapshot, the worktrees whose directory is gone, where the caller stands, the jobs the run daemon knows (with why it could not tell, or that one of another build answered), and wtm's state of the repository, the jobs' logs under it.
type Facts struct {
	worktree.Repository
	repository.Asked
	Gone     map[string]bool // by path
	Cwd      string          // where the caller stands, through its symbolic links
	StateDir string
}

// A Target is a worktree whose jobs run logs reads: its name, its branch ("" on none), its path, and whether it is the main one.
type Target struct {
	Worktree string
	Branch   string
	Path     string
	Main     bool
}

// standing are the worktrees whose directory stands, the main one first.
func (f Facts) standing() []worktree.Worktree {
	return slices.DeleteFunc(f.MainFirst(), func(w worktree.Worktree) bool { return f.Gone[w.Path] })
}

// picked are the worktrees the request names, else the one the caller stands in.
func (f Facts) picked(req Request) ([]string, domain.Origin) {
	if len(req.Worktrees) > 0 {
		return req.Worktrees, domain.OriginRequest
	}
	worktrees := f.standing()
	if at := worktree.Holding(worktrees, f.Cwd, func(w worktree.Worktree) string { return w.Path }); at >= 0 {
		return []string{worktrees[at].Name()}, domain.OriginDefault
	}

	return nil, domain.OriginDefault
}

// targets are the worktrees names name, by their name, and the problems of the names that name none.
func (f Facts) targets(names []string) (targets []Target, problems []domain.FieldError) {
	for index, name := range names {
		path := entryPath(index)
		at := slices.IndexFunc(f.Worktrees, func(w worktree.Worktree) bool { return w.Name() == name })
		switch {
		case strings.TrimSpace(name) == "":
			problems = append(problems, domain.Required(path))
		case slices.Contains(names[:index], name):
			problems = append(problems, domain.FieldError{Path: path, Code: domain.CodeDistinct, Template: "{path}: {value} is given twice", Params: domain.Params{"value": name}})
		case at < 0:
			problems = append(problems, domain.FieldError{Path: path, Code: domain.CodeNotFound, Template: "{value} names no worktree", Params: domain.Params{"value": name}})
		case f.Gone[f.Worktrees[at].Path]:
			problems = append(problems, domain.FieldError{Path: path, Code: CodeWorktreeGone, Template: "{value}'s directory is gone", Params: domain.Params{"value": name, "path": f.Worktrees[at].Path}})
		default:
			w := f.Worktrees[at]
			targets = append(targets, Target{Worktree: w.Name(), Branch: w.Branch(), Path: w.Path, Main: w.Main})
		}
	}
	return targets, problems
}

// main is the main checkout, where a shared job runs for every worktree.
func (f Facts) main() Target {
	for _, w := range f.Worktrees {
		if w.Main {
			return Target{Worktree: w.Name(), Branch: w.Branch(), Path: w.Path, Main: true}
		}
	}
	return Target{}
}

// listed is the job called name the daemon lists in the worktree at dir.
func (f Facts) listed(dir, name string) (run.Job, bool) {
	at := slices.IndexFunc(f.Jobs, func(job run.Job) bool { return job.WorkDir == dir && job.Name == name })
	if at < 0 {
		return run.Job{}, false
	}
	return f.Jobs[at], true
}

// ----------------------------------------------------------------------------
// The sources

// A Source is one job of a worktree run logs reads: the worktree, the worktree it prints in (the main checkout for a shared service), the job, as run.toml declares it and as the daemon lists it there (Listed false when it does not), the job that prints its lines (a claim's is the shared service), how to ask the daemon for them and its log file, whether the request named it, and whether it is a shared service another worktree than the main one may hold.
type Source struct {
	Target   Target
	Home     Target
	Name     string
	Kind     project.JobKind
	Declared project.Job
	Job      run.Job
	Listed   bool
	Service  run.Job // the job that prints its lines: Job itself, a claim's shared service
	Lines    run.Tail
	Log      run.LogFile
	Named    bool
	Claimed  bool
}

// Sources are the jobs req reads in each of targets, in run.toml's order then the others the daemon lists there: the job req names alone, else every job.
func Sources(f Facts, req Request, targets []Target) []Source {
	var sources []Source
	for _, t := range targets {
		for _, name := range f.names(req, t) {
			sources = append(sources, f.source(t, name, req.Job != ""))
		}
	}
	return sources
}

// names are the jobs req reads in t: the one it names, else those run.toml declares, then those the daemon lists there it does not.
func (f Facts) names(req Request, t Target) []string {
	if req.Job != "" {
		return []string{req.Job}
	}
	names := make([]string, 0, len(f.Run.Jobs))
	for _, job := range f.Run.Jobs {
		names = append(names, job.Name)
	}
	for _, job := range f.Jobs {
		if job.WorkDir == t.Path && !slices.Contains(names, job.Name) {
			names = append(names, job.Name)
		}
	}
	return names
}

func (f Facts) source(t Target, name string, named bool) Source {
	declared, _ := f.declared(name)
	job, listed := f.listed(t.Path, name)
	src := Source{Target: t, Name: name, Kind: declared.Kind, Declared: declared, Job: job, Listed: listed, Service: job, Named: named}
	if src.Kind == "" {
		src.Kind = job.Kind
	}

	// A shared job prints in the main checkout, whichever worktree holds it.
	src.Home = t
	if declared.Scope == project.Shared || job.SharedDir != "" {
		src.Home = f.main()
		src.Claimed = !t.Main
		src.Service, _ = f.listed(src.Home.Path, name)
	}
	src.Lines = run.Tail{Name: name, WorkDir: src.Home.Path, Lines: run.MaxTail}
	if dir := run.LogDir(f.StateDir, src.Home.Branch); dir != "" {
		src.Log = run.LogFile{Dir: dir, Job: name}
	}
	return src
}

func (f Facts) declared(name string) (project.Job, bool) {
	at := slices.IndexFunc(f.Run.Jobs, func(job project.Job) bool { return job.Name == name })
	if at < 0 {
		return project.Job{}, false
	}
	return f.Run.Jobs[at], true
}

// Shows reports whether the source is read given that its log holds lines (or cannot be read): a job named, a job the daemon lists, or one that left a log; a shared service only in the main checkout or in a worktree holding it.
func (s Source) Shows(logged bool) bool {
	switch {
	case s.Named, s.Listed:
		return true
	case s.Claimed:
		return false
	}
	return logged
}

// Ran reports whether the source's job ran, given that its log holds lines (or cannot be read): its state is any but not started.
func (s Source) Ran(logged bool) bool {
	return s.state(logged) != JobNotStarted
}

// Follows reports whether the lines of the source are followed: its service is a process of a daemon of this protocol, running.
func (s Source) Follows(f Facts) bool {
	legacy := f.Skewed != nil && f.Skewed.Skew == run.SkewLegacy
	return !legacy && s.Listed && s.Service.Hold == run.HoldOwn && (s.Service.State == run.Running || s.Service.State == run.Starting)
}

// LogPath is the file the source's lines are kept in, "" for a job that keeps none.
func (s Source) LogPath() string {
	if s.Log.Dir == "" {
		return ""
	}
	return filepath.Join(s.Log.Dir, s.Log.Job+".log")
}

// ----------------------------------------------------------------------------
// The detail

// A Detail is one worktree as the JSON envelope reports it: its path, and each job read there, in run.toml's order.
type Detail struct {
	Path string `json:"path"`
	Jobs []Job  `json:"jobs"`
}

// A Job is one job of a worktree: its name and kind, its state, what of it the daemon holds and, for a shared service, the main checkout's worktree it runs in; the code its process exited with, its address, the profile it was started under, the folder it runs in (run.toml's cwd, relative to the worktree) and its command, the file its lines are kept in, the command that follows the containers of a docker compose stack it left running (whose services log to Docker, not to wtm), its lines, and, for a read of the whole log, how many lines the log holds.
type Job struct {
	Name        string          `json:"name"`
	Kind        project.JobKind `json:"kind,omitempty"`
	State       JobState        `json:"state"`
	Hold        run.Hold        `json:"hold,omitempty"`
	SharedIn    string          `json:"shared_in,omitempty"`
	ExitCode    *int            `json:"exit_code,omitempty"`
	URL         string          `json:"url,omitempty"`
	Profile     string          `json:"profile,omitempty"`
	Dir         string          `json:"dir,omitempty"`
	Command     string          `json:"command,omitempty"`
	Log         string          `json:"log,omitempty"`
	ComposeLogs string          `json:"compose_logs,omitempty"`
	Lines       []Line          `json:"lines"`
	Count       int             `json:"count,omitempty"`
}

// A Line is one line a job printed: when (zero for a line of a log that kept no time), its text, and, read from the whole log, its number in it from 1.
type Line struct {
	At   time.Time `json:"at,omitzero"`
	Text string    `json:"text"`
	N    int       `json:"n,omitempty"`
}

// JobState is where a job stands, as run logs reads it.
type JobState string

// The states of a job: the daemon's, and a job it does not list: stopped when it left a log, not started otherwise.
const (
	JobStarting   JobState = "starting"
	JobRunning    JobState = "running"
	JobCrashed    JobState = "crashed"
	JobExited     JobState = "exited"
	JobStopped    JobState = "stopped"
	JobNotStarted JobState = "not_started"
)

// JobStates returns every JobState.
func JobStates() []JobState {
	return []JobState{JobStarting, JobRunning, JobCrashed, JobExited, JobStopped, JobNotStarted}
}

// JobOf is the job source reads, with what was read of its log, whether its log holds a line (or cannot be read), and the project name of its compose stack ("" for none).
func JobOf(s Source, found Found, logged bool, project string) Job {
	job := Job{Name: s.Name, Kind: s.Kind, State: s.state(logged), Dir: s.Declared.Cwd, Command: s.Declared.Cmd, Log: s.LogPath(), Lines: found.Lines, Count: found.Count}
	if job.Lines == nil {
		job.Lines = []Line{}
	}
	if s.Listed {
		job.Hold, job.ExitCode, job.URL, job.Profile = s.Job.Hold, s.Service.ExitCode, cmp.Or(s.Job.URL, s.Service.URL), s.Job.Profile
	}
	if s.Home.Path != s.Target.Path {
		job.SharedIn = s.Home.Worktree
	}
	if s.Compose() {
		job.ComposeLogs = composeLogs(project)
	}
	return job
}

// Compose reports whether the source is a docker compose stack its launcher left running: its containers log to Docker.
func (s Source) Compose() bool {
	return s.Listed && s.Job.Hold == run.HoldDetached && (strings.Contains(s.Declared.Cmd, "docker compose") || strings.Contains(s.Declared.Cmd, "docker-compose"))
}

// composeLogs is the command that follows the containers of a compose project, the one its folder names when project is "".
func composeLogs(project string) string {
	if project == "" {
		return "docker compose logs -f"
	}
	return "docker compose -p " + project + " logs -f"
}

func (s Source) state(logged bool) JobState {
	switch {
	case s.Listed && s.Service.State != "":
		return JobState(s.Service.State)
	case s.Listed && s.Job.Up:
		return JobRunning
	case logged:
		return JobStopped
	}
	return JobNotStarted
}

// ----------------------------------------------------------------------------
// Report

// Logged is what was read of each source, by its index in the sources, or why its log could not be read; and the compose project of each worktree a stack runs in, by its path.
type Logged struct {
	Found      map[int]Found
	Unreadable map[int]error
	Projects   map[string]string
}

// Report is one item per target, its jobs those of sources it shows, given the lines read of each; and the warnings of what could not be asked.
func Report(f Facts, targets []Target, sources []Source, read Logged) ([]Item, []domain.Warning) {
	items := make([]Item, 0, len(targets))
	for _, t := range targets {
		item := Item{Subject: t.Worktree, Status: domain.StatusDone, Detail: &Detail{Path: t.Path, Jobs: []Job{}}}
		for i, s := range sources {
			logged := read.Found[i].Logged || read.Unreadable[i] != nil
			if s.Target.Path != t.Path || !s.Shows(logged) {
				continue
			}
			item.Detail.Jobs = append(item.Detail.Jobs, JobOf(s, read.Found[i], logged, read.Projects[s.Home.Path]))
			if err := read.Unreadable[i]; err != nil {
				item.Warnings = append(item.Warnings, unreadable(s, err))
			}
		}
		items = append(items, item)
	}

	return items, f.Asked.Warnings(Name)
}

// ----------------------------------------------------------------------------
// The follow

// A Resume drops, from the lines a follow started again sends, those the follow before it sent: every line older than Last, the last one it sent, and the first Sent of Last's time (two lines may share a millisecond). Gap says lines were lost between the two: Last is not where it should be among them.
type Resume struct {
	Last    run.Line
	Sent    int
	Gap     bool
	dropped int
	passed  bool
}

// Drops reports whether line was sent before, and is to be dropped.
func (r *Resume) Drops(line run.Line) bool {
	if r.passed {
		return false
	}
	switch {
	case line.At.Before(r.Last.At):
		return true
	case line.At.Equal(r.Last.At) && r.dropped < r.Sent:
		r.dropped++
		if r.dropped == r.Sent {
			r.passed, r.Gap = true, line != r.Last
		}
		return true
	}
	// Last's lines sent before rotated out of what the daemon keeps.
	r.passed, r.Gap = true, true
	return false
}

// ----------------------------------------------------------------------------
// The whole log

// A Matcher tells whether the text of a line is one a search looks for.
type Matcher func(text string) bool

// MatcherOf is the matcher of req's search: its text, or its regular expression with Regex, its case ignored unless MatchCase; nil when req searches nothing. It fails on a regular expression that does not compile.
func MatcherOf(req Request) (Matcher, error) {
	if req.Search == "" {
		return nil, nil
	}
	pattern := req.Search
	if !req.Regex {
		pattern = regexp.QuoteMeta(pattern)
	}
	if !req.MatchCase {
		pattern = "(?i)" + pattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	return re.MatchString, nil
}

func sinceOf(req Request) (time.Time, error) {
	if req.Since == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, req.Since)
}

// Whole reports whether req reads each job's whole log, its backups included, rather than its last lines: it searches it, or reads it from a time.
func (r Request) Whole() bool {
	return r.Search != "" || r.Since != ""
}

// Found is what was read of a job's log: its lines (numbered from 1 in the whole log when it was read whole), how many lines the whole log holds (0 for its last lines only), and whether it holds any.
type Found struct {
	Lines  []Line
	Count  int
	Logged bool
}

// LastOf is found of the last lines of a log.
func LastOf(lines []run.Line) Found {
	found := Found{Lines: make([]Line, 0, len(lines)), Logged: len(lines) > 0}
	for _, line := range lines {
		found.Lines = append(found.Lines, Line{At: line.At, Text: line.Text})
	}
	return found
}

// A Sift keeps what a request reads of a whole log as it goes by, a line after the other: from Since on, the lines its search matches (every line without one); the last run.MaxTail of a search, the first run.MaxTail from a time.
type Sift struct {
	match   Matcher
	since   time.Time
	started bool
	found   Found
}

// NewSift is the sift of req, a request Resolve settled.
func NewSift(req Request) *Sift {
	match, _ := MatcherOf(req)
	since, _ := sinceOf(req)
	return &Sift{match: match, since: since, started: since.IsZero()}
}

// Take takes the next line of the log.
func (s *Sift) Take(line run.Line) {
	s.found.Count++
	s.found.Logged = true
	// a line no stamp dates follows the one before it
	if !s.started && !line.At.IsZero() && !line.At.Before(s.since) {
		s.started = true
	}
	switch {
	case !s.started || s.match != nil && !s.match(line.Text):
	case len(s.found.Lines) < run.MaxTail:
		s.found.Lines = append(s.found.Lines, Line{At: line.At, Text: line.Text, N: s.found.Count})
	case s.match != nil:
		s.found.Lines = append(s.found.Lines[1:], Line{At: line.At, Text: line.Text, N: s.found.Count})
	}
}

// Found is what the sift kept, and how many lines it saw.
func (s *Sift) Found() Found {
	return s.found
}
