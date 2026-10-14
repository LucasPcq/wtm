package kernel_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LucasPcq/wtm/internal/kernel"
)

func TestAValueIsZeroWhenItHoldsNothing(t *testing.T) {
	for _, zero := range []kernel.Value{nil, kernel.Text(""), kernel.Bool(false), kernel.List{}, kernel.Decisions{}} {
		assert.True(t, kernel.IsZero(zero), "%#v", zero)
	}
	for _, set := range []kernel.Value{kernel.Text("x"), kernel.Bool(true), kernel.List{"x"}, kernel.Decisions{"k": "v"}} {
		assert.False(t, kernel.IsZero(set), "%#v", set)
	}
}
