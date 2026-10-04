package infra

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const readerGoneEvery = 10 * time.Millisecond

func TestReaderGoneClosesWhenThePipeLosesItsReader(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	gone := ReaderGone(t.Context(), ReaderGoneParams{File: w, Every: readerGoneEvery})

	select {
	case <-gone:
		t.Fatal("gone while the reader is still there")
	case <-time.After(10 * readerGoneEvery):
	}
	r.Close()
	select {
	case <-gone:
	case <-time.After(2 * time.Second):
		t.Fatal("the reader left and nothing noticed")
	}
}

func TestReaderGoneNeverClosesForAFile(t *testing.T) {
	f, err := os.Create(filepath.Join(t.TempDir(), "out"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 10*readerGoneEvery)
	defer cancel()

	select {
	case <-ReaderGone(ctx, ReaderGoneParams{File: f, Every: readerGoneEvery}):
		t.Fatal("a file has no reader to lose")
	case <-ctx.Done():
	}
}
