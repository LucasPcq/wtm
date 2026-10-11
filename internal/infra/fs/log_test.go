package fs_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/infra/fs"
)

func TestALogReplacesTheOneThePreviousRunLeft(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hooks", "on_create.log")
	writeFixture(t, path, "the previous run\n")

	log, err := fs.OpenLog(path)
	require.NoError(t, err)
	_, err = log.WriteString("this run\n")
	require.NoError(t, err)
	require.NoError(t, log.Close())

	assert.Equal(t, "this run\n", read(t, path))
}
