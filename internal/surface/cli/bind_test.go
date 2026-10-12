package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestAFieldPathBecomesAKebabCaseFlagUnlessRenamed(t *testing.T) {
	w := newWorld(t, worldParams{})

	cmd, _ := w.binding(t, Flag(FlagParams{Path: "from", Name: "source", Short: "s"}))

	for _, name := range []string{"branches", "isolation", "fast-forward", "env", "target-base", "source"} {
		assert.NotNil(t, cmd.Flags().Lookup(name), name)
	}
	assert.Nil(t, cmd.Flags().Lookup("from"))
	assert.Equal(t, "s", cmd.Flags().Lookup("source").Shorthand)
}

func TestAPositionalFieldTakesNoFlag(t *testing.T) {
	w := newWorld(t, worldParams{})

	cmd, _ := w.binding(t, Positional("branches"))

	assert.Nil(t, cmd.Flags().Lookup("branches"))
}

func TestABoolOrADecisionsFieldCannotBePositional(t *testing.T) {
	w := newWorld(t, worldParams{})

	assert.Panics(t, func() { w.binding(t, Positional("fast_forward")) })
	assert.Panics(t, func() { w.binding(t, Positional("env")) })
}

func TestBindingANameTheRegistryDoesNotHoldPanics(t *testing.T) {
	w := newWorld(t, worldParams{})

	assert.PanicsWithValue(t, `cli.Bind: the registry holds no "nope"`, func() { w.Engine.Bind("nope", Doc{}) })
}

func TestAskIsAddedOnlyToACommandWithARememberableField(t *testing.T) {
	w := newWorld(t, worldParams{})

	command, _ := w.binding(t)
	query := w.Engine.Bind(queryName, Doc{Use: queryName})

	assert.NotNil(t, command.Flags().Lookup(domain.FlagAsk))
	assert.Nil(t, query.Flags().Lookup(domain.FlagAsk))
	for _, name := range []string{domain.FlagYes, domain.FlagOutput, domain.FlagDryRun, flagExpectFingerprint} {
		assert.NotNil(t, query.Flags().Lookup(name), name)
	}
}

func TestAnEnumOffersItsValuesToCompletion(t *testing.T) {
	w := newWorld(t, worldParams{})
	cmd, _ := w.binding(t)

	complete, found := cmd.GetFlagCompletionFunc("isolation")

	require.True(t, found)
	values, _ := complete(cmd, nil, "")
	assert.Equal(t, []string{isolated, verbatim}, values)
}

func TestAFieldIsSpelledAsItsFlagOrItsPositional(t *testing.T) {
	w := newWorld(t, worldParams{})
	_, b := w.binding(t, Positional("branches"), Flag(FlagParams{Path: "fast_forward", Name: "ff"}))

	assert.Equal(t, "<branches>", b.spell("branches"))
	assert.Equal(t, "<branches>[2]", b.spell("branches[2]"))
	assert.Equal(t, "--ff", b.spell("fast_forward"))
	assert.Equal(t, "--target-base", b.spell("target.base"))
	assert.Equal(t, "--env[PORT]", b.spell("env[PORT]"))
	assert.Equal(t, "elsewhere", b.spell("elsewhere"))
}

func TestTheDetailDeclaredIsAnnotatedOnTheCommand(t *testing.T) {
	w := newWorld(t, worldParams{})

	generic, _ := w.binding(t, GenericDetail())
	undeclared, _ := w.binding(t)

	assert.Equal(t, detailGeneric, generic.Annotations[annotationDetail])
	assert.Empty(t, undeclared.Annotations[annotationDetail])
	assert.Equal(t, commandName, generic.Annotations[annotationEntry])
}
