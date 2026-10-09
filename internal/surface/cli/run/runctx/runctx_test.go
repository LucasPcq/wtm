package runctx_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/surface/cli/run/runctx"
	"github.com/LucasPcq/wtm/internal/surface/cli/shared"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
	"github.com/LucasPcq/wtm/internal/testutil/globaldir"
)

type project struct {
	dir      string
	stateDir string
}

func newProject(t *testing.T) project {
	t.Helper()
	globaldir.Isolate(t)
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", domain.StateDirName)
	if err := config.WriteProject(config.WriteProjectParams{
		StateDir: stateDir,
		Answers:  domain.InitProjectAnswers{BasePath: "../.trees", BaseBranch: "main", EnvStrategy: domain.EnvStrategyExample},
	}); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return project{dir: dir, stateDir: stateDir}
}

func (p project) declare(t *testing.T) {
	t.Helper()
	if err := config.WriteRun(config.WriteRunParams{StateDir: p.stateDir, Force: true, Config: domain.RunConfig{
		Jobs: []domain.JobConfig{{Name: "api", Kind: domain.JobKindService, Cmd: "pnpm dev"}},
	}}); err != nil {
		t.Fatalf("write run.toml: %v", err)
	}
}

func (p project) corrupt(t *testing.T) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(p.stateDir, domain.RunFileName), []byte("[[job]\nname ="), 0o644); err != nil {
		t.Fatal(err)
	}
}

func command(t *testing.T, args ...string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "probe"}
	cmd.SetContext(t.Context())
	shared.AddOutputFlag(cmd)
	shared.AddYesFlag(cmd, "")
	if err := cmd.ParseFlags(args); err != nil {
		t.Fatal(err)
	}
	return cmd
}

func tty(t *testing.T, answer bool) {
	t.Helper()
	previous := runctx.IsTTY
	runctx.IsTTY = func() bool { return answer }
	t.Cleanup(func() { runctx.IsTTY = previous })
}

func TestOpenReadsTheProjectAndItsRunToml(t *testing.T) {
	p := newProject(t)
	p.declare(t)

	ctx, err := runctx.Open(runctx.OpenParams{Cmd: command(t), Dir: p.dir})

	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if len(ctx.Run.Jobs) != 1 || ctx.Run.Jobs[0].Name != "api" || ctx.Dir != p.dir {
		t.Errorf("ctx = %+v, want api read from %s", ctx, p.dir)
	}
}

// A run module nobody opted into refuses every command but the two that
// create its first entry.
func TestOpenRefusesARunModuleNotInitialized(t *testing.T) {
	p := newProject(t)

	_, err := runctx.Open(runctx.OpenParams{Cmd: command(t), Dir: p.dir})
	if !errors.Is(err, domain.ErrRunNotInitialized) {
		t.Fatalf("err = %v, want ErrRunNotInitialized", err)
	}

	if _, err := runctx.Open(runctx.OpenParams{Cmd: command(t), Dir: p.dir, SkipGuard: true}); err != nil {
		t.Errorf("SkipGuard: %v, want the creation path let through", err)
	}
}

// Stopping must never depend on the file: an unreadable run.toml is kept as a
// reason, not a refusal, for the commands that asked to tolerate it.
func TestOpenToleratesAnUnreadableRunTomlOnlyWhenAsked(t *testing.T) {
	p := newProject(t)
	p.corrupt(t)

	if _, err := runctx.Open(runctx.OpenParams{Cmd: command(t), Dir: p.dir}); err == nil || !strings.Contains(err.Error(), "run.toml") {
		t.Fatalf("err = %v, want run.toml refused", err)
	}

	ctx, err := runctx.Open(runctx.OpenParams{Cmd: command(t), Dir: p.dir, TolerateRunConfig: true})
	if err != nil {
		t.Fatalf("tolerated: %v", err)
	}
	if ctx.RunErr == nil || len(ctx.Run.Jobs) != 0 {
		t.Errorf("ctx = %+v, want the reason kept and nothing read", ctx)
	}
}

func TestOpenOutsideAProjectSaysInitFirst(t *testing.T) {
	globaldir.Isolate(t)
	dir := gittest.InitRepo(t)

	_, err := runctx.Open(runctx.OpenParams{Cmd: command(t), Dir: dir})

	if !errors.Is(err, domain.ErrConfigNotFound) {
		t.Fatalf("err = %v, want ErrConfigNotFound", err)
	}
}

// One gate: a human format, on a terminal, and no --yes.
func TestOpenIsInteractiveOnlyWhenSomeoneCanBeAsked(t *testing.T) {
	p := newProject(t)
	p.declare(t)
	cases := map[string]struct {
		tty  bool
		args []string
		want bool
	}{
		"terminal":       {tty: true, want: true},
		"no terminal":    {tty: false},
		"--yes":          {tty: true, args: []string{"--" + domain.FlagYes}},
		"--output json":  {tty: true, args: []string{"--" + domain.FlagOutput, domain.OutputJSON}},
		"terminal, text": {tty: true, args: []string{"--" + domain.FlagOutput, domain.OutputText}, want: true},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			tty(t, c.tty)
			ctx, err := runctx.Open(runctx.OpenParams{Cmd: command(t, c.args...), Dir: p.dir})
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			if ctx.Interactive != c.want {
				t.Errorf("Interactive = %v, want %v", ctx.Interactive, c.want)
			}
		})
	}
}

// A listing nobody can act on is a listing: the document under JSON, the table
// without a terminal, and the picker only for someone who can answer it.
func TestListingAnswersWithoutThePickerWhenNobodyCanPick(t *testing.T) {
	cases := map[string]struct {
		ctx      runctx.Context
		answered bool
		want     string
	}{
		"json":        {ctx: runctx.Context{Format: domain.OutputJSON}, answered: true, want: "{}"},
		"no terminal": {ctx: runctx.Context{Format: domain.OutputText}, answered: true, want: "table"},
		"interactive": {ctx: runctx.Context{Format: domain.OutputText, Interactive: true}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			cmd := &cobra.Command{}
			var out bytes.Buffer
			cmd.SetOut(&out)

			answered, err := c.ctx.Listing(runctx.ListingParams{
				Cmd:   cmd,
				JSON:  func(w io.Writer) error { _, err := io.WriteString(w, "{}"); return err },
				Table: func(w io.Writer) { _, _ = io.WriteString(w, "table\n") },
			})

			if err != nil || answered != c.answered {
				t.Fatalf("answered = %v, err = %v, want %v", answered, err, c.answered)
			}
			if !strings.Contains(out.String(), c.want) || (c.want == "" && out.Len() != 0) {
				t.Errorf("wrote %q, want %q", out.String(), c.want)
			}
		})
	}
}

func TestFirstArgIsTheSubjectWhenGiven(t *testing.T) {
	if got := runctx.FirstArg(nil); got != "" {
		t.Errorf("FirstArg(nil) = %q", got)
	}
	if got := runctx.FirstArg([]string{"feat/x", "extra"}); got != "feat/x" {
		t.Errorf("FirstArg = %q, want feat/x", got)
	}
}
