package infra

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

type RecentCommitsParams struct {
	WorktreePath string
	Limit        int
}

func RecentCommits(ctx context.Context, params RecentCommitsParams) ([]domain.CommitSummary, error) {
	cmd := Command(ctx, "git", "-C", params.WorktreePath, "log",
		"--max-count="+strconv.Itoa(params.Limit), "--format="+domain.GitLogFormat)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}

	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	commits := make([]domain.CommitSummary, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		fields := strings.SplitN(line, domain.GitLogFieldSep, domain.GitLogFieldCount)
		if len(fields) != domain.GitLogFieldCount {
			continue
		}
		at, _ := time.Parse(time.RFC3339, fields[3])
		commits = append(commits, domain.CommitSummary{
			SHA:     fields[0],
			Subject: fields[1],
			Author:  fields[2],
			At:      at,
		})
	}
	return commits, nil
}

type DiffShortstatParams struct {
	WorktreePath string
}

// DiffShortstat measures uncommitted work, index included.
func DiffShortstat(ctx context.Context, params DiffShortstatParams) (domain.DiffStat, error) {
	cmd := Command(ctx, "git", "-C", params.WorktreePath, "diff", "HEAD", "--shortstat")
	out, err := cmd.Output()
	if err != nil {
		return domain.DiffStat{}, fmt.Errorf("git diff: %w", err)
	}
	return parseShortstat(string(out)), nil
}

// parseShortstat reads "N files changed, N insertions(+), N deletions(-)",
// where each of the three segments may be absent.
func parseShortstat(out string) domain.DiffStat {
	stat := domain.DiffStat{}
	for _, segment := range strings.Split(out, ",") {
		fields := strings.Fields(strings.TrimSpace(segment))
		if len(fields) < 2 {
			continue
		}
		count, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		switch {
		case strings.HasPrefix(fields[1], "file"):
			stat.FilesChanged = count
		case strings.HasPrefix(fields[1], "insertion"):
			stat.Insertions = count
		case strings.HasPrefix(fields[1], "deletion"):
			stat.Deletions = count
		}
	}
	return stat
}

type BranchDiffShortstatParams struct {
	WorktreePath string
	Base         string
	Branch       string
}

// BranchDiffShortstat measures a branch's committed volume against its base
// — the merge-base comparison (three dots), not a direct two-dot diff, so a
// history that has moved on the base side since the branch forked does not
// get attributed to the branch.
func BranchDiffShortstat(ctx context.Context, params BranchDiffShortstatParams) (domain.DiffStat, error) {
	cmd := Command(ctx, "git", "-C", params.WorktreePath, "diff",
		params.Base+"..."+params.Branch, "--shortstat")
	out, err := cmd.Output()
	if err != nil {
		return domain.DiffStat{}, fmt.Errorf("git diff: %w", err)
	}
	return parseShortstat(string(out)), nil
}

type LastFetchAtParams struct {
	ProjectDir string
}

// LastFetchAt dates the last fetch from FETCH_HEAD's mtime. Zero when the
// repository has never fetched — the most misleading case, which the caller
// treats as stale.
func LastFetchAt(ctx context.Context, params LastFetchAtParams) time.Time {
	gitDir, err := GitCommonDir(ctx, GitCommonDirParams{Dir: params.ProjectDir})
	if err != nil {
		return time.Time{}
	}
	info, err := os.Stat(filepath.Join(gitDir, domain.FetchHeadFileName))
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}
