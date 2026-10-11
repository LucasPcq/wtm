package statefile_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainstatefile "github.com/LucasPcq/wtm/internal/domain/statefile"
	"github.com/LucasPcq/wtm/internal/rules/statefile"
)

func TestAFileWithoutASchemaIsAtSchemaOne(t *testing.T) {
	assert.False(t, statefile.Newer(statefile.Schema{Found: 0, Supported: 1}))
	assert.False(t, statefile.Newer(statefile.Schema{Found: 0, Supported: 2}))
}

func TestOnlyAFileAboveWhatThisWtmWritesIsNewer(t *testing.T) {
	assert.False(t, statefile.Newer(statefile.Schema{Found: 1, Supported: 1}))
	assert.False(t, statefile.Newer(statefile.Schema{Found: 1, Supported: 2}))
	assert.True(t, statefile.Newer(statefile.Schema{Found: 2, Supported: 1}))
}

func TestTheSchemaKeyIsWrittenFromTwoOn(t *testing.T) {
	assert.Zero(t, statefile.Written(1))
	assert.Equal(t, 3, statefile.Written(3))
}

func TestAWriteOverANewerFileIsRefusedWithItsSchema(t *testing.T) {
	schema := statefile.Schema{Path: "/repo/.git/wtm/run.toml", Found: 2, Supported: 1}

	err := statefile.Refusal(schema)

	var newer *statefile.NewerError
	require.True(t, errors.As(err, &newer))
	assert.Equal(t, schema, newer.Schema)
	assert.ErrorIs(t, err, domainstatefile.ErrSchemaNewer)
	assert.Equal(t, "/repo/.git/wtm/run.toml is at schema 2, newer than the 1 this wtm writes: upgrade wtm to change it", err.Error())
}

func TestAWriteOverACurrentFileGoesAhead(t *testing.T) {
	assert.NoError(t, statefile.Refusal(statefile.Schema{Path: "run.toml", Found: 1, Supported: 1}))
}
