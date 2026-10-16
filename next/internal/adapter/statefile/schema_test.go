package statefile

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAFileWithoutASchemaIsAtSchemaOne(t *testing.T) {
	assert.False(t, Newer(Schema{Found: 0, Supported: 1}))
	assert.False(t, Newer(Schema{Found: 0, Supported: 2}))
}

func TestOnlyAFileAboveWhatThisWtmWritesIsNewer(t *testing.T) {
	assert.False(t, Newer(Schema{Found: 1, Supported: 1}))
	assert.False(t, Newer(Schema{Found: 1, Supported: 2}))
	assert.True(t, Newer(Schema{Found: 2, Supported: 1}))
}

func TestTheSchemaKeyIsStampedFromTwoOn(t *testing.T) {
	assert.Zero(t, Stamp(1))
	assert.Equal(t, 3, Stamp(3))
}

func TestAWriteOverANewerFileIsRefusedWithItsSchema(t *testing.T) {
	err := Refusal(Schema{Path: "/repo/.git/wtm/run.toml", Found: 2, Supported: 1})

	var newer *wtm.SchemaNewer
	require.True(t, errors.As(err, &newer))
	assert.Equal(t, wtm.SchemaNewer{Path: "/repo/.git/wtm/run.toml", Found: 2, Supported: 1}, *newer)
	assert.ErrorIs(t, err, wtm.ErrSchemaNewer)
	assert.Equal(t, "/repo/.git/wtm/run.toml is at schema 2, newer than the 1 this wtm writes: upgrade wtm to change it", err.Error())
}

func TestAWriteOverACurrentFileGoesAhead(t *testing.T) {
	assert.NoError(t, Refusal(Schema{Path: "run.toml", Found: 1, Supported: 1}))
}
