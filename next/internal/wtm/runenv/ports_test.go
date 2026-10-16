package runenv

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
		{"zero is not a block of nothing", run.Config{PortOffsetBlock: 0}, defaultOffsetBlock},
		{"a negative block is not one either", run.Config{PortOffsetBlock: -5}, defaultOffsetBlock},
		{"declared wins", run.Config{PortOffsetBlock: 100}, 100},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, offsetBlock(tc.config))
		})
	}
}

func TestALifecyclePortOfASharedJobIsNotShifted(t *testing.T) {
	config := run.Config{Jobs: []run.Job{
		{Name: "db", Scope: run.Shared, Ports: map[string]int{"POSTGRES_PORT": 5432}},
		{Name: "web", Ports: map[string]int{"WEB_PORT": 3000}},
	}}

	assert.Equal(t, map[string]int{"POSTGRES_PORT": 5432, "WEB_PORT": 3030}, lifecyclePorts(lifecyclePortsParams{Run: config, Offset: 30}))
}

func TestAPortNameTwoJobsDeclareHasNoLifecycleAnswer(t *testing.T) {
	config := run.Config{Jobs: []run.Job{
		{Name: "api", Ports: map[string]int{"PORT": 4000}},
		{Name: "web", Ports: map[string]int{"PORT": 3000, "HMR_PORT": 24678}},
	}}

	assert.Equal(t, map[string]int{"HMR_PORT": 24688}, lifecyclePorts(lifecyclePortsParams{Run: config, Offset: 10}))
	assert.Nil(t, lifecyclePorts(lifecyclePortsParams{Run: run.Config{}, Offset: 10}))
}

func TestThePortBasesAreKeyedByJobAndName(t *testing.T) {
	config := run.Config{Jobs: []run.Job{
		{Name: "db", Ports: map[string]int{"POSTGRES_PORT": 5432}},
		{Name: "api", Ports: map[string]int{"API_PORT": 3000}},
	}}

	assert.Equal(t, map[portRef]int{{Job: "db", Name: "POSTGRES_PORT"}: 5432, {Job: "api", Name: "API_PORT"}: 3000}, portBases(config))
}

func TestAResolvedPortShiftsUnlessTheJobIsShared(t *testing.T) {
	assert.Equal(t, 5452, resolvedPort(resolvedPortParams{Base: 5432, Offset: 20}))
	assert.Equal(t, 5432, resolvedPort(resolvedPortParams{Base: 5432, Offset: 20, Shared: true}))
}

func TestTheSharedJobsAreThoseScopedShared(t *testing.T) {
	config := run.Config{Jobs: []run.Job{{Name: "db", Scope: run.Shared}, {Name: "web"}}}

	assert.Equal(t, map[string]bool{"db": true}, sharedJobs(config))
}
