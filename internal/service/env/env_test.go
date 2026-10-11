package env

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestExampleStrategy(t *testing.T) {
	main := t.TempDir()
	target := t.TempDir()

	writeTestFile(t, filepath.Join(main, ".env.example"), "DB_HOST=localhost")

	err := CopyEnvFiles(t.Context(), CopyEnvFilesParams{
		Strategy:         domain.EnvStrategyExample,
		Files:            []domain.EnvFile{{Target: ".env", Template: ".env.example"}},
		TargetDir:        target,
		MainWorktreePath: main,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(target, ".env"))
	if err != nil {
		t.Fatal("expected .env to exist in target")
	}
	if string(data) != "DB_HOST=localhost" {
		t.Errorf("unexpected content: %s", data)
	}
}

func TestExampleStrategyPinnedNonExampleTemplate(t *testing.T) {
	main := t.TempDir()
	target := t.TempDir()

	writeTestFile(t, filepath.Join(main, ".env.dist"), "FROM_DIST=yes")

	err := CopyEnvFiles(t.Context(), CopyEnvFilesParams{
		Strategy:         domain.EnvStrategyExample,
		Files:            []domain.EnvFile{{Target: ".env", Template: ".env.dist"}},
		TargetDir:        target,
		MainWorktreePath: main,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(target, ".env"))
	if err != nil {
		t.Fatal("expected .env to exist in target")
	}
	if string(data) != "FROM_DIST=yes" {
		t.Errorf("expected content copied from .env.dist, got: %s", data)
	}
}

func TestExampleStrategyProbesCandidatesWhenTemplateEmpty(t *testing.T) {
	main := t.TempDir()
	target := t.TempDir()

	writeTestFile(t, filepath.Join(main, ".env.sample"), "FROM_SAMPLE=yes")

	err := CopyEnvFiles(t.Context(), CopyEnvFilesParams{
		Strategy:         domain.EnvStrategyExample,
		Files:            []domain.EnvFile{{Target: ".env"}},
		TargetDir:        target,
		MainWorktreePath: main,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(target, ".env"))
	if err != nil {
		t.Fatal("expected .env to exist in target")
	}
	if string(data) != "FROM_SAMPLE=yes" {
		t.Errorf("expected content probed from .env.sample, got: %s", data)
	}
}

func TestExampleStrategyMissingFile(t *testing.T) {
	main := t.TempDir()
	target := t.TempDir()

	err := CopyEnvFiles(t.Context(), CopyEnvFilesParams{
		Strategy:         domain.EnvStrategyExample,
		Files:            []domain.EnvFile{{Target: ".env", Template: ".env.example"}},
		TargetDir:        target,
		MainWorktreePath: main,
	})
	if err != nil {
		t.Fatalf("should not error on missing .env.example: %v", err)
	}

	if _, err := os.Stat(filepath.Join(target, ".env")); !os.IsNotExist(err) {
		t.Error("expected .env to not exist when .env.example is missing")
	}
}

func TestMainStrategy(t *testing.T) {
	main := t.TempDir()
	target := t.TempDir()

	writeTestFile(t, filepath.Join(main, ".env"), "SECRET=abc")

	err := CopyEnvFiles(t.Context(), CopyEnvFilesParams{
		Strategy:         domain.EnvStrategyMain,
		Files:            []domain.EnvFile{{Target: ".env"}},
		TargetDir:        target,
		MainWorktreePath: main,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(target, ".env"))
	if string(data) != "SECRET=abc" {
		t.Errorf("unexpected content: %s", data)
	}
}

func TestParentStrategy(t *testing.T) {
	main := t.TempDir()
	parent := t.TempDir()
	target := t.TempDir()

	writeTestFile(t, filepath.Join(parent, ".env"), "FROM_PARENT=yes")
	writeTestFile(t, filepath.Join(main, ".env"), "FROM_MAIN=yes")

	err := CopyEnvFiles(t.Context(), CopyEnvFilesParams{
		Strategy:           domain.EnvStrategyParent,
		Files:              []domain.EnvFile{{Target: ".env"}},
		TargetDir:          target,
		MainWorktreePath:   main,
		ParentWorktreePath: parent,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(target, ".env"))
	if string(data) != "FROM_PARENT=yes" {
		t.Errorf("expected content from parent, got: %s", data)
	}
}

func TestParentStrategyFallbackToMain(t *testing.T) {
	main := t.TempDir()
	parent := t.TempDir()
	target := t.TempDir()

	writeTestFile(t, filepath.Join(main, ".env"), "FROM_MAIN=yes")

	err := CopyEnvFiles(t.Context(), CopyEnvFilesParams{
		Strategy:           domain.EnvStrategyParent,
		Files:              []domain.EnvFile{{Target: ".env"}},
		TargetDir:          target,
		MainWorktreePath:   main,
		ParentWorktreePath: parent,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(target, ".env"))
	if string(data) != "FROM_MAIN=yes" {
		t.Errorf("expected fallback to main, got: %s", data)
	}
}

func TestNestedEnvFiles(t *testing.T) {
	main := t.TempDir()
	target := t.TempDir()

	writeTestFile(t, filepath.Join(main, "apps", "api", ".env"), "API_KEY=123")

	err := CopyEnvFiles(t.Context(), CopyEnvFilesParams{
		Strategy:         domain.EnvStrategyMain,
		Files:            []domain.EnvFile{{Target: "apps/api/.env"}},
		TargetDir:        target,
		MainWorktreePath: main,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(target, "apps", "api", ".env"))
	if string(data) != "API_KEY=123" {
		t.Errorf("unexpected content: %s", data)
	}
}

func TestEmptyFiles(t *testing.T) {
	err := CopyEnvFiles(t.Context(), CopyEnvFilesParams{
		Strategy:         domain.EnvStrategyExample,
		Files:            nil,
		TargetDir:        t.TempDir(),
		MainWorktreePath: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("empty files should be no-op: %v", err)
	}
}
