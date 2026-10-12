package cli

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestEveryEntryOfTheRegistryIsServedOrDeclaredUnbound(t *testing.T) {
	gaps := parityGaps(parityParams{Registry: production.schema(), Root: Root(), Unbound: unbound})

	assert.Empty(t, gaps)
}

func TestTheExampleIsServedWhole(t *testing.T) {
	w := newWorld(t, worldParams{})

	assert.Empty(t, parityGaps(parityParams{Registry: w.Engine.schema(), Root: w.root()}))
}

func TestAnEntryNoCommandServesIsAGap(t *testing.T) {
	w := newWorld(t, worldParams{})
	root := &cobra.Command{Use: "wtm"}
	root.AddCommand(w.Engine.Bind(commandName, Doc{Use: commandName}, GenericDetail()))

	gaps := parityGaps(parityParams{Registry: w.Engine.schema(), Root: root})

	assert.Equal(t, []string{"inventory is in the registry, but no command serves it and it is not declared unbound"}, gaps)
}

func TestACommandWithNoRenderingDeclaredIsAGap(t *testing.T) {
	w := newWorld(t, worldParams{})
	root := &cobra.Command{Use: "wtm"}
	root.AddCommand(w.Engine.Bind(commandName, Doc{Use: commandName}), w.Engine.Bind(queryName, Doc{Use: queryName}, GenericDetail()))

	gaps := parityGaps(parityParams{Registry: w.Engine.schema(), Root: root})

	assert.Equal(t, []string{"make declares neither its own Detail nor GenericDetail"}, gaps)
}

func TestAnExclusionThatNoLongerHoldsIsAGap(t *testing.T) {
	w := newWorld(t, worldParams{})

	gaps := parityGaps(parityParams{Registry: w.Engine.schema(), Root: w.root(), Unbound: map[string]string{queryName: "later", "gone": "removed"}})

	assert.Equal(t, []string{
		"gone is declared unbound, but the registry does not hold it",
		"inventory is declared unbound, but a command serves it",
	}, gaps)
}
