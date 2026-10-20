package git

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// fieldSep separates the fields of a commit in git log's output: a unit separator, which no subject holds.
const fieldSep = "\x1f"

// Commits returns the last count commits of the worktree at dir, the newest first.
func (r Repo) Commits(ctx context.Context, dir string, count int) ([]worktree.Commit, error) {
	out, err := r.at(dir).git(ctx, "log", "--max-count="+strconv.Itoa(count), "--format=%h"+fieldSep+"%s"+fieldSep+"%an"+fieldSep+"%cI")
	if err != nil {
		return nil, err
	}

	var commits []worktree.Commit
	for line := range strings.Lines(string(out)) {
		fields := strings.SplitN(strings.TrimSuffix(line, "\n"), fieldSep, 4)
		if len(fields) != 4 {
			continue
		}
		at, _ := time.Parse(time.RFC3339, fields[3])
		commits = append(commits, worktree.Commit{SHA: fields[0], Subject: fields[1], Author: fields[2], At: at})
	}

	return commits, nil
}

// Diffstat measures the uncommitted work of the worktree at dir: the tracked files changed in its index and in its working tree, then the lines inserted and deleted against its last commit, index included.
func (r Repo) Diffstat(ctx context.Context, dir string) (worktree.Diffstat, error) {
	at := r.at(dir)
	staged, err := at.git(ctx, "diff", "--cached", "--name-only", "-z")
	if err != nil {
		return worktree.Diffstat{}, err
	}
	unstaged, err := at.git(ctx, "diff", "--name-only", "-z")
	if err != nil {
		return worktree.Diffstat{}, err
	}

	stat := worktree.Diffstat{Staged: strings.Count(string(staged), "\x00"), Unstaged: strings.Count(string(unstaged), "\x00")}
	out, err := at.git(ctx, "diff", "HEAD", "--shortstat")
	if err != nil {
		return stat, err
	}

	// "N files changed, N insertions(+), N deletions(-)", each part left out when it is zero.
	for part := range strings.SplitSeq(string(out), ",") {
		fields := strings.Fields(part)
		if len(fields) < 2 {
			continue
		}
		count, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		switch {
		case strings.HasPrefix(fields[1], "insertion"):
			stat.Insertions = count
		case strings.HasPrefix(fields[1], "deletion"):
			stat.Deletions = count
		}
	}

	return stat, nil
}
