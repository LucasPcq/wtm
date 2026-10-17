package statefile

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAFileWithoutASchemaIsAtSchemaOne(t *testing.T) {
	assert.NoError(t, Refusal(Schema{Found: 0, Supported: 1}))
	assert.NoError(t, Refusal(Schema{Found: 0, Supported: 2}))
}

func TestOnlyAFileAboveWhatThisWtmWritesIsRefused(t *testing.T) {
	assert.NoError(t, Refusal(Schema{Found: 1, Supported: 1}))
	assert.NoError(t, Refusal(Schema{Found: 1, Supported: 2}))
	assert.Error(t, Refusal(Schema{Found: 2, Supported: 1}))
}

func TestAWriteOverANewerFileIsRefusedWithItsSchema(t *testing.T) {
	err := Refusal(Schema{Path: "/repo/.git/wtm/run.toml", Found: 2, Supported: 1})

	newer, ok := errors.AsType[*domain.SchemaNewer](err)
	require.True(t, ok)
	assert.Equal(t, domain.SchemaNewer{Path: "/repo/.git/wtm/run.toml", Found: 2, Supported: 1}, *newer)
	assert.Equal(t, "/repo/.git/wtm/run.toml is at schema 2, newer than the 1 this wtm writes: upgrade wtm to change it", err.Error())
}

func TestAWriteOverACurrentFileGoesAhead(t *testing.T) {
	assert.NoError(t, Refusal(Schema{Path: "run.toml", Found: 1, Supported: 1}))
}
