package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestARequestThatReadsWhereItsCallerStandsIsAskedByIt(t *testing.T) {
	caller := Caller{Dir: "/code/app", Named: true, Cwd: "/code"}

	assert.Equal(t, calledRequest{caller: caller}, CalledBy(calledRequest{}, caller))
	assert.Equal(t, "as is", CalledBy("as is", caller))
}
