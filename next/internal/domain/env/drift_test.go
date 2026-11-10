package env

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/dotenv"
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

func TestEachKeyOfAnEnvFileHasAStatusAgainstItsSources(t *testing.T) {
	keys := Keys(DriftParams{
		Child:    lines("SAME=1\nOTHER=here\nLOCAL=x\nPORT=3010"),
		Template: lines("SAME=\nOTHER=\nSECRET=\nNEW="),
		Main:     lines("SAME=1\nOTHER=main\nNEW=n\nPORT=3000"),
		Owned:    map[string]bool{"PORT": true},
	})

	assert.Equal(t, []Key{
		{Name: "SAME", Status: KeyInStep, Value: "1", Source: "1"},
		{Name: "OTHER", Status: KeyConflict, Value: "here", Source: "main"},
		{Name: "LOCAL", Status: KeyOrphan, Value: "x"},
		{Name: "SECRET", Status: KeyMissing},
		{Name: "NEW", Status: KeyAdd, Source: "n"},
	}, keys)
}

func TestApplyingKeysSettlesEachOnItsSource(t *testing.T) {
	cases := map[string]struct {
		child string
		keys  []Key
		want  string
	}{
		"add_before_trailing_newline": {child: "A=1\n", keys: []Key{{Name: "B", Status: KeyAdd, Source: "2"}}, want: "A=1\nB=2\n"},
		"conflict_keeps_quotes":       {child: "A=\"old\"\n", keys: []Key{{Name: "A", Status: KeyConflict, Value: "old", Source: "new"}}, want: "A=\"new\"\n"},
		"orphan_dropped":              {child: "# c\nA=1\nGONE=x\n", keys: []Key{{Name: "GONE", Status: KeyOrphan, Value: "x"}}, want: "# c\nA=1\n"},
		"missing_untouched":           {child: "A=1\n", keys: []Key{{Name: "S", Status: KeyMissing}}, want: "A=1\n"},
		"crlf_kept":                   {child: "A=1\r\n", keys: []Key{{Name: "B", Status: KeyAdd, Source: "2"}}, want: "A=1\r\nB=2\r\n"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, c.want, string(dotenv.Render(ApplyKeys(dotenv.Parse([]byte(c.child)), c.keys))))
		})
	}
}
