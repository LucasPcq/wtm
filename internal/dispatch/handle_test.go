package dispatch_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LucasPcq/wtm/internal/dispatch"
	"github.com/LucasPcq/wtm/internal/kernel"
)

func TestAPlanWhoseDetailIsNotAStructIsRefusedAtRegistration(t *testing.T) {
	b := bench(t)

	assert.PanicsWithValue(t, "loose: the detail of a plan is a struct, not string", func() {
		dispatch.Register(dispatch.RegisterParams[request, facts, string, string]{Dispatcher: b.Dispatcher, Command: kernel.Command[request, facts, string, string]{Name: "loose"}})
	})
}

func TestTheSchemaIsTheFieldsAsData(t *testing.T) {
	specs := handle(bench(t), &repo{}).Schema()

	paths := make([]string, 0, len(specs))
	for _, spec := range specs {
		paths = append(paths, spec.Path)
	}
	assert.Equal(t, []string{"branches", "from", "isolation"}, paths)
}
