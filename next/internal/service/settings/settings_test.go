package settings

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadingNoConfigIsNone(t *testing.T) {
	saved, err := Service{Store: &store{}}.Read()

	require.NoError(t, err)
	assert.Nil(t, saved)
}

func TestWhatIsWrittenIsReadBack(t *testing.T) {
	service := Service{Store: &store{}}
	rewrite := questions(facts(nil), Given{}).Rewrite()

	_, err := service.Write(t.Context(), rewrite)
	require.NoError(t, err)
	saved, err := service.Read()

	require.NoError(t, err)
	assert.Equal(t, rewrite.Config, saved.Config)
}
