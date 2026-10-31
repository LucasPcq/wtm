package daemon

import (
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
)

// now is the clock of every test's daemon.
var now = time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)

// origin is the repository and worktree the jobs of a test are started from.
var origin = &run.Origin{Root: "/repo", CommonDir: "/repo/.git", Branch: "feat/a", CorrelationID: "start-1"}

func service(name string) run.Spec {
	return run.Spec{Job: project.Job{Name: name, Kind: project.Service, Cmd: "serve " + name, Ports: map[string]int{"PORT": 3000}, URL: &project.URL{Port: "PORT"}}, WorkDir: "/wt/a", Env: map[string]string{"PORT": "3100"}, Origin: origin}
}

func task(name string) run.Spec {
	return run.Spec{Job: project.Job{Name: name, Kind: project.Task, Cmd: "run " + name}, WorkDir: "/wt/a", Origin: origin}
}

func stack(name string) run.Spec {
	return run.Spec{Job: project.Job{Name: name, Kind: project.Service, Cmd: "up " + name, Stop: "down " + name}, WorkDir: "/wt/a", Origin: origin}
}

// A script is what a fake process does once spawned, given what it prints to: nothing, by default, until it is stopped.
type script func(p *fakeProcess, printed func(string))

// exits is a script that prints lines then exits with code.
func exits(code int, lines ...string) script {
	return func(p *fakeProcess, printed func(string)) {
		for _, line := range lines {
			printed(line + "\n")
		}
		p.end(code)
	}
}

// A fakeProcess is a process a test scripts: it exits when its script or a stop says.
type fakeProcess struct {
	pid    int
	once   sync.Once
	exited chan struct{}
	code   int
	stops  []time.Duration
	typed  []byte
	sizes  [][2]int
	print  func(string)
	// afterStop runs once a stop ended the process, before Stop returns: what other clients do while the stopping one waits.
	afterStop func()
	mu        sync.Mutex
}

func (p *fakeProcess) PID() int { return p.pid }

func (p *fakeProcess) Wait() int {
	<-p.exited
	return p.code
}

func (p *fakeProcess) Stop(grace time.Duration) {
	p.mu.Lock()
	p.stops = append(p.stops, grace)
	after := p.afterStop
	p.mu.Unlock()
	p.end(-1)
	<-p.exited
	if after != nil {
		after()
	}
}

func (p *fakeProcess) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.typed = append(p.typed, b...)
	return len(b), nil
}

func (p *fakeProcess) Resize(cols, rows int) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sizes = append(p.sizes, [2]int{cols, rows})
	return nil
}

func (p *fakeProcess) end(code int) {
	p.once.Do(func() {
		p.code = code
		close(p.exited)
	})
}

// A spawner starts fake processes, each running the script its command line is given, and keeps every command and process.
type spawner struct {
	mu        sync.Mutex
	scripts   map[string]script
	commands  []run.Command
	processes map[string]*fakeProcess
	fail      bool
}

func newSpawner(scripts map[string]script) *spawner {
	return &spawner{scripts: scripts, processes: map[string]*fakeProcess{}}
}

func (s *spawner) spawn(command run.Command) (Process, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fail {
		return nil, errSpawn
	}
	p := &fakeProcess{pid: 1000 + len(s.commands), exited: make(chan struct{}), print: func(text string) { command.Output([]byte(text)) }}
	s.commands = append(s.commands, command)
	s.processes[command.Line] = p
	if do := s.scripts[command.Line]; do != nil {
		go do(p, p.print)
	}
	return p, nil
}

func (s *spawner) process(line string) *fakeProcess {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.processes[line]
}

func (s *spawner) lines() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var lines []string
	for _, command := range s.commands {
		lines = append(lines, command.Line)
	}
	return lines
}

type spawnError struct{}

func (spawnError) Error() string { return "no such file" }

var errSpawn = spawnError{}

// orphans answers each group with the state a test gives it and keeps the groups reaped.
type orphans struct {
	groups map[int]run.Group
	reaped []int
}

func (o *orphans) Probe(started map[int]time.Time) map[int]run.Group {
	answer := map[int]run.Group{}
	for pgid := range started {
		answer[pgid] = o.groups[pgid]
	}
	return answer
}

func (o *orphans) Reap(pgids []int) {
	o.reaped = append(o.reaped, pgids...)
	slices.Sort(o.reaped)
}

// index is an index in memory.
type index struct {
	mu      sync.Mutex
	records []run.Record
	// stall, while set, holds each Save until it is closed: a job's end waiting on its index; stalled counts the Saves it holds.
	stall   chan struct{}
	stalled int
}

func (i *index) Load() []run.Record {
	i.mu.Lock()
	defer i.mu.Unlock()
	return slices.Clone(i.records)
}

func (i *index) Save(records []run.Record) error {
	i.mu.Lock()
	stall := i.stall
	if stall != nil {
		i.stalled++
	}
	i.mu.Unlock()
	if stall != nil {
		<-stall
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	i.records = records
	return nil
}

func (i *index) names() []string {
	i.mu.Lock()
	defer i.mu.Unlock()
	names := []string{}
	for _, record := range i.records {
		names = append(names, record.Spec.WorkDir+":"+record.Spec.Job.Name)
	}
	return names
}

// bus keeps the lines a daemon publishes.
type bus struct {
	mu    sync.Mutex
	lines []domain.Line
}

func (b *bus) publish(line domain.Line) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.lines = append(b.lines, line)
}

func (b *bus) events() []domain.Event {
	b.mu.Lock()
	defer b.mu.Unlock()
	var events []domain.Event
	for _, line := range b.lines {
		events = append(events, line.Event)
	}
	return events
}

// prints is a script that prints lines then runs until it is stopped.
func prints(lines ...string) script {
	return func(_ *fakeProcess, printed func(string)) {
		for _, line := range lines {
			printed(line + "\n")
		}
	}
}

// logs is the logs of jobs in memory: each file opened, by its directory and job, with what was written to it.
type logs struct {
	mu     sync.Mutex
	files  map[run.LogFile][]run.Line
	opened []run.LogFile
}

func newLogs() *logs {
	return &logs{files: map[run.LogFile][]run.Line{}}
}

func (l *logs) open(file run.LogFile) (Log, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.opened = append(l.opened, file)
	key := run.LogFile{Dir: file.Dir, Job: file.Job}
	if !file.Append {
		l.files[key] = nil
	}
	return &memoryLog{logs: l, key: key}, nil
}

func (l *logs) read(file run.LogFile, n int) []run.Line {
	l.mu.Lock()
	defer l.mu.Unlock()
	lines := l.files[run.LogFile{Dir: file.Dir, Job: file.Job}]
	return slices.Clone(lines[max(0, len(lines)-n):])
}

func (l *logs) texts(dir, job string) []string {
	var texts []string
	for _, line := range l.read(run.LogFile{Dir: dir, Job: job}, run.MaxTail) {
		texts = append(texts, line.Text)
	}
	return texts
}

type memoryLog struct {
	logs *logs
	key  run.LogFile
}

func (m *memoryLog) Write(lines []run.Line) {
	m.logs.mu.Lock()
	defer m.logs.mu.Unlock()
	m.logs.files[m.key] = append(m.logs.files[m.key], lines...)
}

func (m *memoryLog) Close() {}

// shared is the shared job name of the worktree at workDir, its main checkout /repo: a stack when stop is set, a foreground service otherwise.
func shared(name, workDir, stop string) run.Spec {
	slug := strings.ReplaceAll(strings.TrimPrefix(workDir, "/wt/"), "/", "_")
	spec := run.Spec{
		Job:     project.Job{Name: name, Kind: project.Service, Cmd: "up " + name, Stop: stop, Scope: project.Shared},
		WorkDir: workDir,
		Env:     map[string]string{"WTM_WORKTREE": slug, "WTM_BRANCH": "feat/" + slug, "WTM_ISOLATION": "isolated"},
		Origin:  &run.Origin{Root: "/repo", CommonDir: "/repo/.git", Branch: "feat/" + slug},
		Shared:  &run.Shared{WorkDir: "/repo", Env: map[string]string{"WTM_WORKTREE": "main", "WTM_BRANCH": "main"}},
	}
	if workDir == "/repo" {
		spec.Env, spec.Origin.Branch = spec.Shared.Env, "main"
	}
	return spec
}

// failsFirst is a script that exits 1 the first times times it runs, saying why, then 0.
func failsFirst(times int, why string) script {
	var mu sync.Mutex
	ran := 0
	return func(p *fakeProcess, printed func(string)) {
		mu.Lock()
		ran++
		failing := ran <= times
		mu.Unlock()
		if failing {
			printed(why + "\n")
			p.end(1)
			return
		}
		p.end(0)
	}
}

// holds is a script that runs until release is closed, then exits with code.
func holds(release <-chan struct{}, code int) script {
	return func(p *fakeProcess, _ func(string)) {
		<-release
		p.end(code)
	}
}

// stalling holds every Save from now until the function it returns is called.
func (i *index) stalling() func() {
	i.mu.Lock()
	defer i.mu.Unlock()
	stall := make(chan struct{})
	i.stall = stall
	return func() { close(stall) }
}

// holding reports how many Saves the stall holds.
func (i *index) holding() int {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.stalled
}

// routes is the proxy's table of names in memory.
type routes struct {
	mu     sync.Mutex
	served map[string]run.Address
	port   int
}

func (r *routes) Port() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.port
}

// listen has the proxy serve on port.
func (r *routes) listen(port int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.port = port
}

func (r *routes) Add(address run.Address) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.served == nil {
		r.served = map[string]run.Address{}
	}
	r.served[address.Host] = address
}

func (r *routes) Remove(address run.Address) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.served[address.Host] == address {
		delete(r.served, address.Host)
	}
}

// table returns each host served, mapped to its port.
func (r *routes) table() map[string]int {
	r.mu.Lock()
	defer r.mu.Unlock()
	table := map[string]int{}
	for host, address := range r.served {
		table[host] = address.Port
	}
	return table
}

// routed is spec served under the name of its job in the worktree feat-a of acme, on its port PORT.
func routed(spec run.Spec) run.Spec {
	spec.Env = maps.Clone(spec.Env)
	if spec.Env == nil {
		spec.Env = map[string]string{}
	}
	spec.Env["PORT"], spec.Env["WTM_WORKTREE"], spec.Env["WTM_PROJECT"] = "3100", "feat-a", "acme"
	spec.Job.Ports = map[string]int{"PORT": 3000}
	spec.Job.URL = &project.URL{Port: "PORT"}
	spec.Routes = []run.Route{{Job: spec.Job.Name, Host: spec.Job.Name + ".feat-a.acme.localhost", Port: "PORT"}}
	return spec
}
