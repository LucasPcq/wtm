package output

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	update      = flag.Bool("update", false, "rewrite the golden files")
	testdata, _ = filepath.Abs("testdata")
)

func golden(t *testing.T, name string, got string) {
	t.Helper()
	path := filepath.Join(testdata, name+".golden")
	if *update {
		require.NoError(t, os.WriteFile(path, []byte(got), 0o644))
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, string(want), got)
}

func rendered(draw func(w *bytes.Buffer)) string {
	var out bytes.Buffer
	draw(&out)
	return out.String()
}

func jsonOf(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)
	return string(data)
}

const (
	time2s = 2 * time.Second
	tick   = 10 * time.Millisecond
)
