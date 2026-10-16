package wt

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/spf13/pflag"

	"github.com/LucasPcq/wtm/internal/dispatch"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/registry"
	"github.com/LucasPcq/wtm/internal/surface/cli/render"
	"github.com/LucasPcq/wtm/internal/surface/cli/shared"
)

// createThroughEngine runs `create` for the tests of the other commands, which
// only need a worktree: create is on the engine, served by package cli, which
// imports this one. It answers with the v2 envelope create writes in JSON.
func createThroughEngine(t *testing.T, args []string) (string, string, error) {
	t.Helper()
	flags := pflag.NewFlagSet(domain.CmdCreate, pflag.ContinueOnError)
	from := flags.String(domain.FlagFrom, "", "")
	envFrom := flags.String(domain.FlagEnvFrom, "", "")
	isolation := flags.String(domain.FlagIsolation, "", "")
	fastForward := flags.Bool(domain.FlagFF, false, "")
	ifNotExists := flags.Bool(domain.FlagIfNotExists, false, "")
	ask := flags.Bool(domain.FlagAsk, false, "")
	flags.BoolP(domain.FlagYes, "y", false, "")
	flags.String(domain.FlagOutput, "", "")
	if err := flags.Parse(args); err != nil {
		return "", "", err
	}
	request, err := json.Marshal(map[string]any{
		"branches": flags.Args(), "from": *from, "env_from": *envFrom, "isolation": *isolation,
		"fast_forward": *fastForward, "if_not_exists": *ifNotExists,
	})
	if err != nil {
		return "", "", err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", "", err
	}
	root, err := shared.ProjectRoot(t.Context(), cwd)
	if err != nil {
		return "", "", err
	}
	stateDir, err := shared.StateDir(t.Context(), cwd)
	if err != nil {
		return "", "", err
	}
	production := registry.Production(dispatch.New(dispatch.Params{Dir: root, StateDir: stateDir}))
	entry, _ := production.Lookup(domain.CmdCreate)
	reply, err := entry.Call(t.Context(), dispatch.Call{Request: request, Options: dispatch.Options{IgnoreRemembered: *ask}})
	if err != nil {
		return "", "", err
	}
	applied, _ := reply.(dispatch.CommandReply)
	failure := applied.Outcome.Failure()
	var stdout bytes.Buffer
	if err := render.WriteEnvelope(&stdout, render.EnvelopeParams{
		Command: domain.CmdCreate, OK: failure == nil, Op: applied.Op, Items: applied.Outcome.Items,
		FollowUps: applied.Outcome.FollowUps, Warnings: applied.Outcome.Warnings, Err: failure,
		Wording: render.Wording{Catalog: production.Catalog()},
	}); err != nil {
		return "", "", err
	}
	if failure != nil {
		return stdout.String(), "", failure
	}
	return stdout.String(), "", nil
}

// decodeCreated reads the one worktree a v2 envelope of create reports.
func decodeCreated(t *testing.T, stdout string) domain.CreateResult {
	t.Helper()
	var envelope struct {
		Items []struct {
			Subject  string          `json:"subject"`
			Reason   string          `json:"reason"`
			Detail   json.RawMessage `json:"detail"`
			Warnings []struct {
				Message string `json:"message"`
			} `json:"warnings"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("decode create JSON: %v (payload %q)", err, stdout)
	}
	if len(envelope.Items) != 1 {
		t.Fatalf("items = %+v, want exactly one", envelope.Items)
	}
	var created domain.CreateResult
	if err := json.Unmarshal(envelope.Items[0].Detail, &created); err != nil {
		t.Fatalf("decode create detail: %v", err)
	}
	item := envelope.Items[0]
	created.Branch, created.AlreadyExists, created.Isolation = item.Subject, item.Reason == "unchanged", created.Metadata.Isolation
	for _, warning := range item.Warnings {
		created.Warnings = append(created.Warnings, warning.Message)
	}
	return created
}

// fieldCodes names the code of each field an invalid create was refused on.
func fieldCodes(t *testing.T, err error) map[string]kernel.Code {
	t.Helper()
	var invalid *kernel.InvalidError
	if !errors.As(err, &invalid) {
		t.Fatalf("err = %v, want an invalid request", err)
	}
	codes := map[string]kernel.Code{}
	for _, field := range invalid.Fields {
		codes[field.Path] = field.Code
	}
	return codes
}
