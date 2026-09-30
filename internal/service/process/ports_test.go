package process

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestJobPortsUsesTheInjectedOffset(t *testing.T) {
	job := domain.JobConfig{Name: "web", Ports: map[string]int{"PORT": 3000}}

	main := jobPorts(job, map[string]string{domain.EnvPortOffset: "0"})
	if main["PORT"] != 3000 {
		t.Errorf("main checkout keeps its base, got %v", main)
	}

	second := jobPorts(job, map[string]string{domain.EnvPortOffset: "10"})
	if second["PORT"] != 3010 {
		t.Errorf("got %v, want PORT=3010", second)
	}
}

// An env with no offset is a run that never resolved one. Falling back to zero
// keeps the declared ports rather than shifting them by a garbage number.
func TestJobPortsWithoutOffset(t *testing.T) {
	job := domain.JobConfig{Name: "web", Ports: map[string]int{"PORT": 3000}}
	for _, env := range []map[string]string{nil, {}, {domain.EnvPortOffset: "nonsense"}} {
		if got := jobPorts(job, env); got["PORT"] != 3000 {
			t.Errorf("env %v: got %v, want PORT=3000", env, got)
		}
	}
}

func TestWithJobPortsOverridesInheritedValue(t *testing.T) {
	job := domain.JobConfig{Name: "web", Ports: map[string]int{"PORT": 3000}}
	env := map[string]string{domain.EnvPortOffset: "10", "PORT": "4000"}

	merged := withJobPorts(job, env)
	if merged["PORT"] != "3010" {
		t.Errorf("run.toml must win over an inherited value, got %v", merged)
	}
	if env["PORT"] != "4000" {
		t.Errorf("the caller's map must not be mutated, got %v", env)
	}
}

func TestWithJobPortsLeavesADeclarationlessJobAlone(t *testing.T) {
	env := map[string]string{domain.EnvPortOffset: "10"}
	if got := withJobPorts(domain.JobConfig{Name: "web"}, env); len(got) != 1 {
		t.Errorf("got %v, want the worktree env untouched", got)
	}
}

func TestManagerStartTask_InjectsResolvedPorts(t *testing.T) {
	m := newManager()
	dir := t.TempDir()

	var buf bytes.Buffer
	job := domain.JobConfig{
		Name:  "ports",
		Kind:  domain.JobKindTask,
		Cmd:   "printenv PORT",
		Ports: map[string]int{"PORT": 3000},
	}

	if err := m.Start(StartParams{
		Job:      job,
		WorkDir:  dir,
		Env:      map[string]string{domain.EnvPortOffset: "10"},
		Streamer: &buf,
	}); err != nil {
		t.Fatalf("start task: %v", err)
	}

	if got := strings.TrimSpace(buf.String()); got != "3010" {
		t.Errorf("task saw PORT=%q, want 3010", got)
	}
}

// The ports a detached service bound must survive to its stop command, or a
// templated `docker compose down` tears down a stack it never brought up.
func TestManagerStop_RunsStopCommandWithResolvedPorts(t *testing.T) {
	m := newManager()
	dir := t.TempDir()
	marker := filepath.Join(dir, "stopped-port")

	script := filepath.Join(dir, "stop.sh")
	body := "#!/bin/sh\nprintenv DB_PORT > " + marker + "\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	job := domain.JobConfig{
		Name:  "db",
		Kind:  domain.JobKindService,
		Cmd:   "true",
		Stop:  script,
		Ports: map[string]int{"DB_PORT": 5432},
	}

	if err := m.Start(StartParams{
		Job:      job,
		WorkDir:  dir,
		Env:      map[string]string{domain.EnvPortOffset: "10"},
		Streamer: &bytes.Buffer{},
	}); err != nil {
		t.Fatalf("start service: %v", err)
	}
	if err := m.Stop(JobRef{Name: job.Name, WorkDir: dir}); err != nil {
		t.Fatalf("stop service: %v", err)
	}

	seen, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("stop command did not run: %v", err)
	}
	if got := strings.TrimSpace(string(seen)); got != "5442" {
		t.Errorf("stop command saw DB_PORT=%q, want 5442", got)
	}
}
