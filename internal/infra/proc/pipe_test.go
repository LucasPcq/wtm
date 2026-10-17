package proc

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestReaderGoneClosesWhenThePipeLosesItsReader(t *testing.T) {
	r, w, err := os.Pipe()
	require.NoError(t, err)
	defer w.Close()
	gone := ReaderGone(t.Context(), ReaderGoneParams{File: w, Every: readerGoneEvery})

	select {
	case <-gone:
		require.FailNow(t, "gone while the reader is still there")
	case <-time.After(10 * readerGoneEvery):
	}
	r.Close()
	select {
	case <-gone:
	case <-time.After(2 * time.Second):
		require.FailNow(t, "the reader left and nothing noticed")
	}
}

func TestReaderGoneNeverClosesForAFile(t *testing.T) {
	f, err := os.Create(filepath.Join(t.TempDir(), "out"))
	require.NoError(t, err)
	defer f.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 10*readerGoneEvery)
	defer cancel()

	select {
	case <-ReaderGone(ctx, ReaderGoneParams{File: f, Every: readerGoneEvery}):
		require.FailNow(t, "a file has no reader to lose")
	case <-ctx.Done():
	}
}
