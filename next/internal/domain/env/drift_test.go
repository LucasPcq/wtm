package env

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/project"

	"github.com/stretchr/testify/assert"
)

func TestTheDriftOfAnEnvFileCountsItsKeysAgainstItsSources(t *testing.T) {
	cases := map[string]struct {
		params DriftParams
		drift  Drift
	}{
		"in_line": {
			params: DriftParams{Child: lines("A=1\nB=2"), Template: lines("A=\nB="), Main: lines("A=1\nB=2")},
		},
		"missing_with_no_value_anywhere": {
			params: DriftParams{Child: lines("A=1"), Template: lines("A=\nSECRET=")},
			drift:  Drift{Missing: 1},
		},
		"absent_but_in_main_is_no_gap": {
			params: DriftParams{Child: lines("A=1"), Main: lines("A=1\nB=2")},
		},
		"conflicting_with_its_source": {
			params: DriftParams{Child: lines("A=1"), Parent: lines("A=2"), Main: lines("A=1")},
			drift:  Drift{Conflicting: 1},
		},
		"orphan": {
			params: DriftParams{Child: lines("A=1\nLOCAL=x"), Main: lines("A=1")},
			drift:  Drift{Orphan: 1},
		},
		"owned_is_never_drift": {
			params: DriftParams{Child: lines("PORT=3010\nWTM_ORDINAL=1"), Main: lines("PORT=3000"), Owned: Owned(project.Run{EnvPorts: []project.EnvPortLink{{File: ".env", Key: "PORT"}}}, ".env")},
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, c.drift, KeyDrift(c.params))
		})
	}
}

func TestTheOwnedKeysAreThoseOfTheirFileOnly(t *testing.T) {
	run := project.Run{
		EnvPorts:  []project.EnvPortLink{{File: ".env", Key: "PORT"}, {File: "web/.env", Key: "WEB_PORT"}},
		EnvValues: []project.EnvValueLink{{File: ".env", Key: "REALM"}},
	}

	owned := Owned(run, ".env")

	assert.True(t, owned["PORT"])
	assert.True(t, owned["REALM"])
	assert.True(t, owned[ComposeProjectKey])
	assert.False(t, owned["WEB_PORT"])
}
