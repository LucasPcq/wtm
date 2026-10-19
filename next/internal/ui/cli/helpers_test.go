package cli

import (
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
	"github.com/spf13/cobra"
)

type printed struct {
	Stdout string
	Stderr string
	Code   int
}

func runIn(t *testing.T, args ...string) printed {
	t.Helper()
	streams, _, stdout, stderr := output.Test()
	root := NewRoot("1.2.3", streams, []*cobra.Command{probe()})
	code := Execute(t.Context(), root, args)
	return printed{Stdout: stdout.String(), Stderr: stderr.String(), Code: code}
}

func realPath(t *testing.T, path string) string {
	t.Helper()
	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return resolvedPath
}

// rerun is the follow-up that runs create again on branch.
func rerun(branch string) domain.FollowUp {
	return domain.FollowUp{Command: "create", Answers: []domain.Answered{{Path: "branches", Values: []string{branch}}}}
}
