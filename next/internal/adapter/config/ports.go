package config

import (
	"fmt"
	"maps"
	"regexp"
	"slices"

	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
)

const (
	portMin = 1
	portMax = 65535
)

var envVarName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// declaration is one port a job of run.toml declares, with its base value.
type declaration struct {
	Job    string
	Name   string
	Base   int
	Shared bool
}

// portProblems lists every port that is not a valid environment variable name
// or lies outside the port range, then every pair of ports that collide.
func (f runFile) portProblems() []string {
	var problems []string
	for _, decl := range f.declarations() {
		if !envVarName.MatchString(decl.Name) {
			problems = append(problems, fmt.Sprintf("job %q: port %q is not a valid environment variable name", decl.Job, decl.Name))
		}
		if decl.Base < portMin || decl.Base > portMax {
			problems = append(problems, fmt.Sprintf("job %q: port %s is %d, outside %d-%d", decl.Job, decl.Name, decl.Base, portMin, portMax))
		}
	}

	return append(problems, f.collisions()...)
}

// collisions lists the pairs of ports that would bind the same port in some
// worktree: each worktree shifts a per-worktree port by a multiple of the
// offset block, while a shared port never moves.
func (f runFile) collisions() []string {
	block := env.OffsetBlock(project.Run{PortOffsetBlock: f.PortOffsetBlock})
	decls := f.declarations()

	var problems []string
	for i, a := range decls {
		for _, b := range decls[i+1:] {
			gap := max(b.Base-a.Base, a.Base-b.Base)
			meets := gap%block == 0
			if a.Shared && b.Shared {
				meets = gap == 0
			}

			apart := gap / block
			switch {
			case !meets || apart > env.CollisionHorizon:
			case apart == 0:
				problems = append(problems, fmt.Sprintf("ports %s (job %q) and %s (job %q) both declare base %d — they would bind the same port in every worktree", a.Name, a.Job, b.Name, b.Job, a.Base))
			default:
				problems = append(problems, fmt.Sprintf("ports %s (job %q, base %d) and %s (job %q, base %d) collide %d worktree(s) apart with a port_offset_block of %d — move one base so the gap between them is not a multiple of %d", a.Name, a.Job, a.Base, b.Name, b.Job, b.Base, apart, block, block))
			}
		}
	}
	return problems
}

// declarations returns every port of every job, in job order and, within a
// job, sorted by name so the problems read the same on every run.
func (f runFile) declarations() []declaration {
	var decls []declaration
	for _, job := range f.Jobs {
		for _, name := range slices.Sorted(maps.Keys(job.Ports)) {
			decls = append(decls, declaration{Job: job.Name, Name: name, Base: job.Ports[name], Shared: job.Scope == string(project.Shared)})
		}
	}
	return decls
}
