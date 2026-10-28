package main

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/clean"
	"github.com/LucasPcq/wtm/next/internal/command/exec"
	"github.com/LucasPcq/wtm/next/internal/command/prune"

	"github.com/LucasPcq/wtm/next/internal/command/events"
	"github.com/LucasPcq/wtm/next/internal/command/list"
	"github.com/LucasPcq/wtm/next/internal/command/status"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
	"github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
	"github.com/spf13/cobra"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const commandPrefix = "github.com/LucasPcq/wtm/next/internal/command/"

func TestEachRegisteredCommandIsInTheCatalogueOrItsStreamsAndWired(t *testing.T) {
	registered := registeredCommands(t)
	var listed, streamed []string
	for _, command := range catalogue() {
		// a command's folder spells its name without dashes: command/fastforward is fast-forward
		listed = append(listed, strings.ReplaceAll(command.Name, "-", ""))
	}
	for _, stream := range streams() {
		streamed = append(streamed, stream.Name)
		assert.NotNil(t, stream.Watch, stream.Name)
	}
	// shortcut: a non-zero use case is taken as wired, not checked field by field; a test that runs each command would prove more
	wired := map[string]bool{}
	app := reflect.ValueOf(wire(wireParams{MainPath: t.TempDir()}))
	for i := range app.NumField() {
		if name, ok := strings.CutPrefix(app.Field(i).Type().PkgPath(), commandPrefix); ok && !app.Field(i).IsZero() {
			wired[name] = true
		}
	}

	require.NotEmpty(t, registered)
	assert.Equal(t, streamsListed, streamed, "catalogue.go streams a command the list of streams does not name, or the reverse")
	assert.ElementsMatch(t, registered, append(listed, streamed...), "main.go registers a command catalogue.go does not list, or the reverse")
	for _, name := range registered {
		// a stream follows every repository: wire.go builds it once, not per repository in app
		if !slices.Contains(streamsListed, name) {
			assert.True(t, wired[name], "%s is registered in main.go but wire.go builds no %s.UseCase", name, name)
		}
	}
}

// A request that carries where its caller stands and forgot From would be asked from nowhere by every surface, silently.
func TestEachRequestThatCarriesItsCallerTakesItFromEverySurface(t *testing.T) {
	callerType := reflect.TypeFor[domain.Caller]()
	var requests []reflect.Type
	app := reflect.ValueOf(wire(wireParams{MainPath: t.TempDir()}))
	for i := range app.NumField() {
		run, ok := app.Field(i).Type().MethodByName("Run")
		require.True(t, ok, app.Type().Field(i).Name)
		requests = append(requests, run.Type.In(2))
	}
	// a stream is built once, out of app: each one of streamsListed by its use case
	streamed := map[string]reflect.Type{events.Name: reflect.TypeFor[events.UseCase]()}
	for _, name := range streamsListed {
		useCase, ok := streamed[name]
		require.True(t, ok, "stream %s has no use case here", name)
		watch, ok := useCase.MethodByName("Watch")
		require.True(t, ok, name)
		requests = append(requests, watch.Type.In(2))
	}

	carrying := 0
	for _, req := range requests {
		if !slices.ContainsFunc(reflect.VisibleFields(req), func(f reflect.StructField) bool { return f.Type == callerType }) {
			continue
		}
		carrying++
		from, ok := req.MethodByName("From")
		assert.True(t, ok && from.Type.NumIn() == 2 && from.Type.In(1) == callerType && from.Type.NumOut() == 1 && from.Type.Out(0) == req, "%s carries a domain.Caller but is no domain.ReadsCaller", req)
	}
	assert.Positive(t, carrying, "status and events carry their caller")
}

func TestEachFollowUpOfAMigratedCommandIsSpelledFromItsSchema(t *testing.T) {
	streams, _, _, _ := output.Test()
	root := cli.NewRoot(version, streams, commands(streams, cli.Env{}))
	followUps := []domain.FollowUp{leftBehind("feat/a"), *initialize(), settle("feat/a"), proxyStatus()}
	followUps = append(followUps, repository.Offered(list.Actions(list.Detail{Branch: "feat/a"}))...)
	fixes := status.Problems(status.ProblemsParams{Branch: "feat/a", Merged: true, Missing: []status.Missing{{Target: ".env"}, {Target: ".env", From: status.FromNowhere}}, Jobs: []status.Job{{Name: "api", State: run.Crashed}}})
	for _, problem := range fixes {
		followUps = append(followUps, *problem.Fix)
	}
	for _, followUp := range followUps {
		require.NotEmpty(t, followUp.Command)
		_, listed := catalogue().Find(followUp.Command)
		if !listed && !runs(root, strings.Fields(followUp.Command)) {
			continue
		}

		require.Contains(t, []string{clean.Name, prune.Name, exec.Name}, followUp.Command, "%s has migrated: spell its follow-ups from its schema in commandLine", followUp.Command)
		argv := commandLine(followUp)
		assert.Equal(t, []string{cli.Program, followUp.Command}, argv[:min(2, len(argv))], "%s's follow-up %v does not fold into its request", followUp.Command, followUp.Answers)
	}
}

func TestTheGuardTellsARunSubcommandFromAnother(t *testing.T) {
	root, run := &cobra.Command{Use: "wtm-next"}, &cobra.Command{Use: "run"}
	run.AddCommand(&cobra.Command{Use: "init", Run: func(*cobra.Command, []string) {}})
	root.AddCommand(run)

	assert.True(t, runs(root, []string{"run", "init"}))
	assert.False(t, runs(root, []string{"run", "up"}))
	assert.False(t, runs(root, []string{"clean"}))
}
