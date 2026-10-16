package jobenv

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/wtm/run"
	"github.com/stretchr/testify/assert"
)

func TestTheOffsetBlockIsTheDeclaredOneElseTheDefault(t *testing.T) {
	cases := []struct {
		name   string
		config run.Config
		want   int
	}{
		{"unset falls back to the default", run.Config{}, defaultOffsetBlock},
		{"a negative block is not one either", run.Config{PortOffsetBlock: -5}, defaultOffsetBlock},
		{"declared wins", run.Config{PortOffsetBlock: 100}, 100},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, OffsetBlock(tc.config))
		})
	}
}

func TestALifecyclePortOfASharedJobIsNotShifted(t *testing.T) {
	config := run.Config{PortOffsetBlock: 10, Jobs: []run.Job{
		{Name: "db", Scope: run.Shared, Ports: map[string]int{"POSTGRES_PORT": 5432}},
		{Name: "web", Ports: map[string]int{"WEB_PORT": 3000}},
	}}

	assert.Equal(t, map[string]int{"POSTGRES_PORT": 5432, "WEB_PORT": 3030}, Params{Ordinal: 3, Run: config}.ports())
}

func TestAPortNameTwoJobsDeclareHasNoLifecycleAnswer(t *testing.T) {
	config := run.Config{PortOffsetBlock: 10, Jobs: []run.Job{
		{Name: "api", Ports: map[string]int{"PORT": 4000}},
		{Name: "web", Ports: map[string]int{"PORT": 3000, "HMR_PORT": 24678}},
	}}

	assert.Equal(t, map[string]int{"HMR_PORT": 24688}, Params{Ordinal: 1, Run: config}.ports())
	assert.Empty(t, Params{Ordinal: 1}.ports())
}

func TestAResolvedPortShiftsUnlessTheJobIsShared(t *testing.T) {
	assert.Equal(t, 5452, ResolvedPort(ResolvedPortParams{Base: 5432, Offset: 20}))
	assert.Equal(t, 5432, ResolvedPort(ResolvedPortParams{Base: 5432, Offset: 20, Shared: true}))
}

func TestTheSharedJobsAreThoseScopedShared(t *testing.T) {
	assert.Equal(t, map[string]bool{"db": true}, SharedJobs(run.Config{Jobs: []run.Job{{Name: "db", Scope: run.Shared}, {Name: "web"}}}))
}
