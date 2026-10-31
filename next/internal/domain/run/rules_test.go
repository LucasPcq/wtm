package run

import (
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain/project"

	"github.com/stretchr/testify/assert"
)

func TestTheIndexIsReadAtItsVersionFrozenAtANewerAndReplacedAtAnOlder(t *testing.T) {
	cases := map[string]struct {
		file         int
		read, frozen bool
	}{
		"own":   {file: IndexVersion, read: true},
		"newer": {file: IndexVersion + 1, frozen: true},
		"older": {file: IndexVersion - 1},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			read, frozen := IndexAccess(c.file)

			assert.Equal(t, c.read, read)
			assert.Equal(t, c.frozen, frozen)
		})
	}
}

func TestARecordADaemonLeftIsReapedCrashedKeptOrDropped(t *testing.T) {
	service := Record{Spec: Spec{Job: project.Job{Name: "web", Kind: project.Service}}}
	detached := Record{Spec: Spec{Job: project.Job{Name: "db", Kind: project.Service, Stop: "docker compose down"}}}
	claim := Record{Claim: true, Spec: Spec{Job: project.Job{Name: "db", Kind: project.Service, Stop: "docker compose down"}}}
	task := Record{Spec: Spec{Job: project.Job{Name: "migrate", Kind: project.Task}}}
	cases := map[string]struct {
		params ReconcileParams
		want   Decision
	}{
		"orphan_reaped":     {params: ReconcileParams{Record: service, Group: Group{Alive: true, Identified: true}}, want: Decision{Keep: true, State: Stopped, Reaped: true}},
		"stranger_dropped":  {params: ReconcileParams{Record: service, Group: Group{Alive: true}, WorkDirExists: true}, want: Decision{}},
		"dead_crashed":      {params: ReconcileParams{Record: service, WorkDirExists: true}, want: Decision{Keep: true, State: Crashed}},
		"dead_gone_dropped": {params: ReconcileParams{Record: service}, want: Decision{}},
		"detached_kept":     {params: ReconcileParams{Record: detached, WorkDirExists: true}, want: Decision{Keep: true, State: Running}},
		"detached_gone":     {params: ReconcileParams{Record: detached}, want: Decision{}},
		"claim_kept":        {params: ReconcileParams{Record: claim, WorkDirExists: true}, want: Decision{Keep: true}},
		"claim_gone":        {params: ReconcileParams{Record: claim}, want: Decision{}},
		"task_dropped":      {params: ReconcileParams{Record: task, WorkDirExists: true}, want: Decision{}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, c.want, Reconcile(c.params))
		})
	}
}

func TestADaemonSkewsByProtocolThenByBuild(t *testing.T) {
	cases := map[string]struct {
		stamp Stamp
		want  Skew
	}{
		"same":     {stamp: Stamp{Protocol: ProtocolVersion, Version: "1.0"}, want: SkewNone},
		"build":    {stamp: Stamp{Protocol: ProtocolVersion, Version: "0.9"}, want: SkewBuild},
		"protocol": {stamp: Stamp{Protocol: ProtocolVersion + 1, Version: "1.0"}, want: SkewProtocol},
		"legacy":   {stamp: Stamp{Version: "1.0"}, want: SkewLegacy},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, c.want, SkewOf(c.stamp, "1.0"))
		})
	}
}

func TestOnlyASkewedDaemonHoldingNoJobGivesWay(t *testing.T) {
	assert.False(t, Replaceable(SkewNone, 0))
	assert.True(t, Replaceable(SkewBuild, 0))
	assert.True(t, Replaceable(SkewLegacy, 0))
	assert.False(t, Replaceable(SkewBuild, 2))
}

func TestTheSplitterCutsOutputIntoCleanLines(t *testing.T) {
	at := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	var s Splitter

	first := s.Split([]byte("\x1b[32mready\x1b[0m in 3ms\r\nhalf"), at)
	second := s.Split([]byte(" a line\n10%\r50%\r100%\n\n\x07bell\n"), at)
	last := s.Flush(at)

	assert.Equal(t, []Line{{At: at, Text: "ready in 3ms"}}, first)
	assert.Equal(t, []Line{{At: at, Text: "half a line"}, {At: at, Text: "100%"}, {At: at, Text: "bell"}}, second)
	assert.Empty(t, last)
}

func TestTheSplitterGivesOutALineThatNeverEnds(t *testing.T) {
	var s Splitter
	at := time.Now()

	lines := s.Split(make([]byte, maxPending+1), at)
	lines = append(lines, s.Split([]byte("tail"), at)...)

	assert.Empty(t, lines, "a line of NULs is blank once cleaned")
	assert.Equal(t, []Line{{At: at, Text: "tail"}}, s.Flush(at))
}

func TestTheSpecsPortsAreThoseItsEnvironmentSpells(t *testing.T) {
	spec := Spec{Job: project.Job{Ports: map[string]int{"PORT": 3000, "DEBUG_PORT": 9229, "GONE": 1}}, Env: map[string]string{"PORT": "3100", "DEBUG_PORT": "9329", "GONE": "x"}}

	assert.Equal(t, map[string]int{"PORT": 3100, "DEBUG_PORT": 9329}, spec.Ports())
}
