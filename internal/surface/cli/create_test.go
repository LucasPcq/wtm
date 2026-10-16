package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/command/create"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/registry"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/surface/cli/shared"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

const createConfig = "[worktrees]\nbase_path = \"../.trees\"\nbase_branch = \"main\"\n\n[env]\nstrategy = \"example\"\n\n[hooks]\non_create = []\n"

type createEnvelope struct {
	OK    bool `json:"ok"`
	Items []struct {
		Subject string `json:"subject"`
		Status  string `json:"status"`
		Reason  string `json:"reason"`
		Detail  struct {
			Path string `json:"path"`
		} `json:"detail"`
	} `json:"items"`
	Error *struct {
		Kind   string `json:"kind"`
		Code   string `json:"code"`
		Fields []struct {
			Path string `json:"path"`
			Code string `json:"code"`
			Flag string `json:"flag"`
		} `json:"fields"`
	} `json:"error"`
}

func TestCreateConcludesWithEachWorktreeAndTheWayIntoIt(t *testing.T) {
	dir := createProject(t)

	got := runCreate(t, "feat/a", "feat/b", "--from", "main")

	require.Equal(t, 0, got.Code, got.Stderr)
	assert.Contains(t, got.Stdout, "create: 2 done")
	assert.Contains(t, got.Stdout, "wtm go feat/a")
	assert.Contains(t, got.Stdout, "wtm go feat/b")
	assert.DirExists(t, treeOf(dir, "feat/a"))
}

func TestCreateInJSONIsTheEnvelopeAndNeedsNoYes(t *testing.T) {
	dir := createProject(t)

	got := runCreate(t, "feat/a", "--output", domain.OutputJSON)

	require.Equal(t, 0, got.Code, got.Stderr)
	envelope := decodeEnvelope(t, got.Stdout)
	assert.True(t, envelope.OK)
	require.Len(t, envelope.Items, 1)
	assert.Equal(t, "feat/a", envelope.Items[0].Subject)
	assert.Equal(t, "done", envelope.Items[0].Status)
	assert.Equal(t, treeOf(dir, "feat/a"), envelope.Items[0].Detail.Path)
}

func TestCreateWithoutABranchIsAUsageErrorNamingTheArgument(t *testing.T) {
	createProject(t)

	got := runCreate(t)

	assert.Equal(t, domain.ExitCodeUsage, got.Code)
	assert.Contains(t, got.Stderr, "<branches>")
}

func TestCreateOfABranchOpenElsewhereExits2WithItsReasonInTheEnvelope(t *testing.T) {
	dir := createProject(t)
	gittest.CreateBranch(t, dir, "feat/taken")
	gittest.Git(t, dir, "worktree", "add", filepath.Join(t.TempDir(), "taken"), "feat/taken")

	got := runCreate(t, "feat/taken", "--output", domain.OutputJSON)

	assert.Equal(t, domain.ExitCodeUsage, got.Code)
	envelope := decodeEnvelope(t, got.Stdout)
	require.NotNil(t, envelope.Error)
	require.Len(t, envelope.Error.Fields, 1)
	assert.Equal(t, "branches[0]", envelope.Error.Fields[0].Path)
	assert.Equal(t, string(create.CodeCheckedOutElsewhere), envelope.Error.Fields[0].Code)
}

func TestCreateAlreadyOpenUnderIfNotExistsIsUnchanged(t *testing.T) {
	dir := createProject(t)
	gittest.CreateBranch(t, dir, "feat/taken")
	gittest.Git(t, dir, "worktree", "add", filepath.Join(t.TempDir(), "taken"), "feat/taken")

	got := runCreate(t, "feat/taken", "--if-not-exists", "--output", domain.OutputJSON)

	require.Equal(t, 0, got.Code, got.Stderr)
	assert.Equal(t, "unchanged", decodeEnvelope(t, got.Stdout).Items[0].Reason)
}

func TestCreateWithoutConfigExitsOnItsOwnCode(t *testing.T) {
	createProject(t)
	require.NoError(t, os.Remove(filepath.Join(os.Getenv(domain.EnvStateDir), domain.ConfigFileName)))

	got := runCreate(t, "feat/a")

	assert.Equal(t, domain.ExitCodeConfigNotFound, got.Code)
	assert.Contains(t, got.Stderr, "wtm init")
}

func TestCreateDryRunShowsThePlanAndAppliesNothing(t *testing.T) {
	dir := createProject(t)

	got := runCreate(t, "feat/a", "--dry-run")

	require.Equal(t, 0, got.Code, got.Stderr)
	assert.Contains(t, got.Stdout, "Branches")
	assert.Contains(t, got.Stdout, "fingerprint")
	assert.Contains(t, got.Stdout, "--expect-fingerprint")
	assert.NoDirExists(t, treeOf(dir, "feat/a"))
}

func TestCreateRefusesAnIsolationItDoesNotKnow(t *testing.T) {
	createProject(t)

	got := runCreate(t, "feat/a", "--isolation", "keep")

	assert.Equal(t, domain.ExitCodeUsage, got.Code)
}

func TestCreateTakesTheFastForwardAsFF(t *testing.T) {
	createProject(t)

	got := runCreate(t, "feat/a", "--ff")

	assert.Equal(t, 0, got.Code, got.Stderr)
}

func createProject(t *testing.T) string {
	t.Helper()
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", domain.StateDirName)
	t.Setenv(domain.EnvProjectDir, dir)
	t.Setenv(domain.EnvStateDir, stateDir)
	t.Setenv(domain.EnvGoFile, "")
	require.NoError(t, os.MkdirAll(stateDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, domain.ConfigFileName), []byte(createConfig), 0o644))
	return dir
}

func treeOf(dir, branch string) string {
	return filepath.Join(dir, "..", ".trees", rules.SanitizeBranchName(branch))
}

func runCreate(t *testing.T, words ...string) ran {
	t.Helper()
	stdinWas := shared.StdinIsTerminal
	shared.StdinIsTerminal = func() bool { return false }
	t.Cleanup(func() {
		shared.StdinIsTerminal = stdinWas
		humanOutputSilenced = false
	})
	engine := NewEngine(EngineParams{Registry: registry.Production, Dispatcher: projectDispatcher})
	root := &cobra.Command{Use: domain.AppName, SilenceErrors: true, SilenceUsage: true}
	root.PersistentFlags().BoolP(domain.FlagQuiet, "q", false, "")
	root.AddGroup(&cobra.Group{ID: domain.CmdGroupWorktrees, Title: domain.CmdGroupWorktreesTitle})
	root.AddCommand(newCreateCmd(engine.Bind))
	markUsageErrors(root)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(append([]string{domain.CmdCreate}, words...))
	code := execute(t.Context(), executeParams{Root: root, Stderr: &stderr})
	return ran{Stdout: stdout.String(), Stderr: stderr.String(), Code: code}
}

func decodeEnvelope(t *testing.T, stdout string) createEnvelope {
	t.Helper()
	var envelope createEnvelope
	require.NoError(t, json.Unmarshal([]byte(stdout), &envelope), stdout)
	return envelope
}
