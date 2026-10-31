package daemon

import (
	"slices"
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
	mu     sync.Mutex
}

func (p *fakeProcess) PID() int { return p.pid }

func (p *fakeProcess) Wait() int {
	<-p.exited
	return p.code
}

func (p *fakeProcess) Stop(grace time.Duration) {
	p.mu.Lock()
	p.stops = append(p.stops, grace)
	p.mu.Unlock()
	p.end(-1)
	<-p.exited
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
	p := &fakeProcess{pid: 1000 + len(s.commands), exited: make(chan struct{})}
	s.commands = append(s.commands, command)
	s.processes[command.Line] = p
	if do := s.scripts[command.Line]; do != nil {
		go do(p, func(text string) { command.Output([]byte(text)) })
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

// index is an index in memory, frozen or not.
type index struct {
	mu      sync.Mutex
	records []run.Record
	frozen  bool
	saves   int
}

func (i *index) Load() ([]run.Record, bool) {
	i.mu.Lock()
	defer i.mu.Unlock()
	return slices.Clone(i.records), i.frozen
}

func (i *index) Save(records []run.Record) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.saves++
	if !i.frozen {
		i.records = records
	}
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
