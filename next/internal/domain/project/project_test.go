package project

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTheProxyPortIsTheConfiguredOneElseTheDefaultUnlessSwitchedOff(t *testing.T) {
	cases := []struct {
		name   string
		global Global
		want   int
	}{
		{"absent: the default", Global{}, defaultProxyPort},
		{"a chosen port", Global{Proxy: Proxy{Port: 5000}}, 5000},
		{"switched off", Global{Proxy: Proxy{Enabled: new(false), Port: 5000}}, 0},
		{"switched on without a port", Global{Proxy: Proxy{Enabled: new(true)}}, defaultProxyPort},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.global.ProxyPort())
		})
	}
}

func TestAProxyPortASocketCannotBindIsAProblem(t *testing.T) {
	for port, valid := range map[int]bool{0: true, 1: true, 11080: true, 65535: true, -1: false, 65536: false, 70000: false} {
		err := Proxy{Port: port}.Problem()

		assert.Equal(t, valid, err == nil, "port %d: %v", port, err)
	}
	assert.EqualError(t, Proxy{Port: 70000}.Problem(), "invalid [proxy] port 70000 in config.toml — use 1-65535, or leave it out for 11080")
}

func TestOnlyTheThreeEnvStrategiesAreValid(t *testing.T) {
	for _, strategy := range EnvStrategies() {
		assert.True(t, strategy.Valid(), strategy)
	}
	assert.False(t, EnvStrategy("copy").Valid())
}

func TestAnEmptyEnvStrategyIsNotGivenRatherThanAValue(t *testing.T) {
	assert.False(t, EnvStrategy("").Valid())
}

func TestTheListIsACopyACallerMayChange(t *testing.T) {
	strategies := EnvStrategies()
	strategies[0] = "copy"

	assert.Equal(t, EnvExample, EnvStrategies()[0])
}

func TestEachRememberedQuestionOffersItsAnswers(t *testing.T) {
	cases := map[string][]string{
		RememberEnvStrategy:  {"config", "example", "main", "parent"},
		RememberIsolation:    {"isolated", "verbatim"},
		RememberSourceUpdate: {SourceUpdateFastForward, SourceUpdateKeep},
	}
	for id, want := range cases {
		t.Run(id, func(t *testing.T) {
			values, known := RememberableValues(id)

			assert.True(t, known)
			assert.Equal(t, want, values)
		})
	}
}

func TestAQuestionNothingRemembersHasNoAnswers(t *testing.T) {
	values, known := RememberableValues("delete")

	assert.False(t, known)
	assert.Empty(t, values)
}

func TestTheRememberedIdsAndValuesAreTheOnesWtmWrites(t *testing.T) {
	assert.Equal(t, "env_strategy", RememberEnvStrategy)
	assert.Equal(t, "isolation", RememberIsolation)
	assert.Equal(t, "source_update", RememberSourceUpdate)
	assert.Equal(t, "ff", SourceUpdateFastForward)
	assert.Equal(t, "keep", SourceUpdateKeep)
}

func TestTheSourceUpdateRememberedSaysWhetherToFastForward(t *testing.T) {
	assert.Equal(t, SourceUpdateFastForward, SourceUpdate(true))
	assert.Equal(t, SourceUpdateKeep, SourceUpdate(false))
}
