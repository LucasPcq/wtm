package env

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/dotenv"
	"github.com/LucasPcq/wtm/next/internal/domain/project"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReconcilingSettlesTheKeysThenWhatWtmWrites(t *testing.T) {
	params := linkedResolve()
	writes, err := Resolve(params)
	require.NoError(t, err)

	reconciled := Reconcile(ReconcileParams{
		Reads:    []FileRead{{File: project.EnvFile{Target: ".env"}, Child: lines("PORT=3000\nLOCAL=x\n"), Main: lines("PORT=3000\nAPI_KEY=k\n")}},
		Strategy: project.EnvMain,
		Run:      params.Run,
		Writes:   writes,
	})

	require.Len(t, reconciled.Files, 1)
	file := reconciled.Files[0]
	assert.Equal(t, []Key{{Name: "API_KEY", Status: KeyAdd, Source: "k"}}, file.Settled)
	assert.Equal(t, []Key{{Name: "LOCAL", Status: KeyOrphan, Value: "x"}}, file.Left)
	assert.Contains(t, string(dotenv.Render(file.After)), "PORT=3020\nLOCAL=x\nAPI_KEY=k\n")
	assert.True(t, file.Changed())
	assert.Equal(t, StatusRewrite, reconciled.Ports.Entries[0].Status)
}

func TestReconcilingOverwritesAndPrunesOnlyWhenAsked(t *testing.T) {
	read := FileRead{File: project.EnvFile{Target: ".env"}, Child: lines("A=here\nLOCAL=x\n"), Main: lines("A=main\n")}
	cases := map[string]struct {
		overwrite, prune bool
		want             string
	}{
		"neither":   {want: "A=here\nLOCAL=x\n"},
		"overwrite": {overwrite: true, want: "A=main\nLOCAL=x\n"},
		"prune":     {prune: true, want: "A=here\n"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			reconciled := Reconcile(ReconcileParams{Reads: []FileRead{read}, Strategy: project.EnvMain, Overwrite: c.overwrite, Prune: c.prune})

			assert.Equal(t, c.want, string(dotenv.Render(reconciled.Files[0].After)))
			assert.Equal(t, c.overwrite || c.prune, reconciled.Files[0].Changed())
		})
	}
}

func TestAMissingFileIsRebuiltFromItsScaffold(t *testing.T) {
	cases := map[string]struct {
		strategy project.EnvStrategy
		read     FileRead
		created  bool
		want     string
	}{
		"example":         {strategy: project.EnvExample, read: FileRead{Template: lines("A=\n"), Main: lines("A=1\n")}, created: true, want: "A=\n"},
		"main":            {strategy: project.EnvMain, read: FileRead{Template: lines("A=\n"), Main: lines("A=1\n")}, created: true, want: "A=1\n"},
		"parent_fallback": {strategy: project.EnvParent, read: FileRead{Main: lines("A=1\n")}, created: true, want: "A=1\n"},
		"nothing":         {strategy: project.EnvMain, read: FileRead{Template: lines("A=\n")}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			c.read.File = project.EnvFile{Target: ".env"}

			file := Reconcile(ReconcileParams{Reads: []FileRead{c.read}, Strategy: c.strategy}).Files[0]

			assert.Equal(t, c.created, file.Created)
			assert.Equal(t, c.want, string(dotenv.Render(file.After)))
		})
	}
}

func TestAVerbatimWorktreeComparesItsLinkedKeysWithTheirSource(t *testing.T) {
	params := linkedResolve()
	read := FileRead{File: project.EnvFile{Target: ".env"}, Child: lines("PORT=3020\n"), Main: lines("PORT=3000\n")}

	reconciled := Reconcile(ReconcileParams{Reads: []FileRead{read}, Strategy: project.EnvMain, Run: params.Run, Verbatim: true, Overwrite: true})

	assert.Equal(t, "PORT=3000\n", string(dotenv.Render(reconciled.Files[0].After)))
}

func TestAFileWtmOwnsAValueInIsCreatedWhenMissing(t *testing.T) {
	writes := Writes{owned: []OwnedEntry{{File: "infra/.env", Key: ComposeProjectKey, Value: "shop_feat_a"}}}

	file := Reconcile(ReconcileParams{Reads: []FileRead{{File: project.EnvFile{Target: "infra/.env"}}}, Strategy: project.EnvMain, Writes: writes}).Files[0]

	assert.True(t, file.Created)
	assert.Equal(t, "COMPOSE_PROJECT_NAME=shop_feat_a", string(dotenv.Render(file.After)))
}

func TestTheMainCheckoutSpelledWithPortsIsTold(t *testing.T) {
	cases := map[string]struct {
		plan Plan
		want bool
	}{
		"port_to_name":   {plan: Plan{Entries: []Entry{{Status: StatusRewrite, Addressing: project.AddressNames, CurrentValue: "http://localhost:3000"}}}, want: true},
		"already_named":  {plan: Plan{Entries: []Entry{{Status: StatusUnchanged, Addressing: project.AddressNames, CurrentValue: "http://web.main.shop.localhost:11080"}}}},
		"port_only_link": {plan: Plan{Entries: []Entry{{Status: StatusRewrite, Addressing: project.AddressPorts, CurrentValue: "3000"}}}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, c.want, c.plan.SpelledByPorts())
		})
	}
}

func TestAWorktreeSwitchedToVerbatimGetsTheSourceValuesOfWhatWtmWrote(t *testing.T) {
	params := linkedResolve()
	read := FileRead{File: project.EnvFile{Target: ".env"}, Child: lines("PORT=3020\nCOMPOSE_PROJECT_NAME=shop_feat_a\nMINE=x\n"), Main: lines("PORT=3000\nCOMPOSE_PROJECT_NAME=shop\nMINE=y\n")}

	file := Reconcile(ReconcileParams{Reads: []FileRead{read}, Strategy: project.EnvMain, Run: params.Run, Verbatim: true, Restore: true}).Files[0]

	assert.Equal(t, "PORT=3000\nCOMPOSE_PROJECT_NAME=shop\nMINE=x\n", string(dotenv.Render(file.After)))
	assert.Len(t, file.Restored, 2)
	assert.Empty(t, file.Settled)
	assert.Equal(t, []Key{{Name: "MINE", Status: KeyConflict, Value: "x", Source: "y"}}, file.Left)
}

func TestAWorktreeSwitchedToVerbatimFromItsTemplateGetsTheMainValues(t *testing.T) {
	// the proof: a worktree whose values come from its template kept the ports wtm had written when switched to verbatim
	params := linkedResolve()
	read := FileRead{File: project.EnvFile{Target: ".env"}, Child: lines("PORT=3020\n"), Template: lines("PORT=\n"), Main: lines("PORT=3000\n")}

	file := Reconcile(ReconcileParams{Reads: []FileRead{read}, Strategy: project.EnvExample, Run: params.Run, Verbatim: true, Restore: true}).Files[0]

	assert.Equal(t, "PORT=3000\n", string(dotenv.Render(file.After)))
}
