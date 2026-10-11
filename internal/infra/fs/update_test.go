package fs

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
)

const (
	envUpdateHelperPath   = "WTM_TEST_UPDATE_HELPER_PATH"
	envUpdateHelperRounds = "WTM_TEST_UPDATE_HELPER_ROUNDS"
)

func increment(t testing.TB, path string) Lock {
	t.Helper()
	lock, err := UpdateFile(t.Context(), UpdateFileParams{
		Path: path,
		Mutate: func(current []byte) ([]byte, error) {
			count := 0
			if current != nil {
				parsed, err := strconv.Atoi(string(current))
				if err != nil {
					return nil, err
				}
				count = parsed
			}
			return []byte(strconv.Itoa(count + 1)), nil
		},
	})
	if err != nil {
		t.Fatalf("UpdateFile: %v", err)
	}
	return lock
}

func counter(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	count, err := strconv.Atoi(string(data))
	if err != nil {
		t.Fatalf("parse %q: %v", data, err)
	}
	return count
}

func TestUpdateFileLosesNoConcurrentUpdate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "counter")
	const writers = 32

	var wg sync.WaitGroup
	for range writers {
		wg.Go(func() { increment(t, path) })
	}
	wg.Wait()

	if got := counter(t, path); got != writers {
		t.Errorf("counter = %d after %d updates", got, writers)
	}
}

func TestUpdateFileLosesNoUpdateAcrossProcesses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "counter")
	const processes, rounds = 4, 25

	var wg sync.WaitGroup
	errs := make(chan error, processes)
	for range processes {
		wg.Go(func() {
			cmd := exec.Command(os.Args[0], "-test.run=^TestUpdateFileHelper$")
			cmd.Env = append(os.Environ(), envUpdateHelperPath+"="+path, envUpdateHelperRounds+"="+strconv.Itoa(rounds))
			if out, err := cmd.CombinedOutput(); err != nil {
				errs <- errors.New(string(out))
			}
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("helper process: %v", err)
	}

	if got := counter(t, path); got != processes*rounds {
		t.Errorf("counter = %d after %d updates from %d processes", got, processes*rounds, processes)
	}
}

func TestUpdateFileHelper(t *testing.T) {
	path := os.Getenv(envUpdateHelperPath)
	if path == "" {
		t.Skip("helper process for TestUpdateFileLosesNoUpdateAcrossProcesses")
	}
	rounds, err := strconv.Atoi(os.Getenv(envUpdateHelperRounds))
	if err != nil {
		t.Fatalf("rounds: %v", err)
	}
	for range rounds {
		increment(t, path)
	}
}

func TestUpdateFileHandsNilForAMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent")
	var seen []byte
	called := false
	if _, err := UpdateFile(t.Context(), UpdateFileParams{Path: path, Mutate: func(current []byte) ([]byte, error) {
		called, seen = true, current
		return []byte("x"), nil
	}}); err != nil {
		t.Fatalf("UpdateFile: %v", err)
	}
	if !called || seen != nil {
		t.Errorf("Mutate saw %q (called %v), want nil", seen, called)
	}
}

func TestUpdateFileCreatesWithItsPermAndKeepsAnExistingMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	write := func(perm os.FileMode, data string) {
		t.Helper()
		if _, err := UpdateFile(t.Context(), UpdateFileParams{Path: path, Perm: perm, Mutate: func([]byte) ([]byte, error) {
			return []byte(data), nil
		}}); err != nil {
			t.Fatalf("UpdateFile: %v", err)
		}
	}
	modeOf := func() os.FileMode {
		t.Helper()
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		return info.Mode().Perm()
	}

	write(0o600, "a")
	if got := modeOf(); got != 0o600 {
		t.Fatalf("created with %o, want 600", got)
	}
	write(0o644, "b")
	if got := modeOf(); got != 0o600 {
		t.Errorf("rewritten with %o, want the existing 600", got)
	}
}

func TestUpdateFileLeavesUnchangedContentAlone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte("same"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := UpdateFile(t.Context(), UpdateFileParams{Path: path, Mutate: func(current []byte) ([]byte, error) {
		return current, nil
	}}); err != nil {
		t.Fatalf("UpdateFile: %v", err)
	}

	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) {
		t.Error("an unchanged file was replaced")
	}
}

func TestUpdateFileRemovesOnRequest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := UpdateFile(t.Context(), UpdateFileParams{Path: path, Mutate: func([]byte) ([]byte, error) {
			return nil, ErrRemoveFile
		}}); err != nil {
			t.Fatalf("UpdateFile: %v", err)
		}
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("file still there: %v", err)
	}
}

func TestUpdateFileWritesNothingWhenMutateFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte("kept"), 0o644); err != nil {
		t.Fatal(err)
	}
	want := errors.New("refused")
	if _, err := UpdateFile(t.Context(), UpdateFileParams{Path: path, Mutate: func([]byte) ([]byte, error) {
		return []byte("lost"), want
	}}); !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
	if data, _ := os.ReadFile(path); string(data) != "kept" {
		t.Errorf("content = %q, want kept", data)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() != "file" && entry.Name() != "file"+lockSuffix {
			t.Errorf("left %s behind", entry.Name())
		}
	}
}

func TestUpdateFileWritesUnprotectedWhenItCannotLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	if err := os.Mkdir(path+lockSuffix, 0o755); err != nil {
		t.Fatal(err)
	}
	if lock := increment(t, path); lock.Held {
		t.Error("Held = true for a lock that could not be taken")
	}
	if got := counter(t, path); got != 1 {
		t.Errorf("counter = %d, want 1", got)
	}
}
