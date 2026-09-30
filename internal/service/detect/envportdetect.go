package detect

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

type ScanEnvPortsParams struct {
	ProjectDir string
	Files      []domain.EnvFile
}

// ScanEnvPorts reads the port declarations of every directory holding env
// files, keyed by that directory relative to the project root — the same key a
// job's cwd carries, which is what pairs a scan with its job.
//
// Within a directory the strongest source wins: .env.local over .env over the
// committed template. Reading the template last-resort is what lets a fresh
// clone, whose .env files are gitignored, still declare its ports.
func ScanEnvPorts(params ScanEnvPortsParams) map[string]domain.EnvPortScan {
	byDir := map[string][]rules.EnvPortSource{}
	failures := map[string]string{}

	for _, file := range params.Files {
		dir := filepath.Dir(file.Target)
		for _, candidate := range sourceFilesFor(file) {
			ports, err := readEnvPorts(filepath.Join(params.ProjectDir, candidate))
			if err != nil {
				failures[dir] = fmt.Sprintf(domain.EnvPortUnreadable, candidate, err)
				continue
			}
			if len(ports) == 0 {
				continue
			}
			byDir[dir] = append(byDir[dir], rules.EnvPortSource{File: candidate, Ports: ports})
		}
	}

	scans := make(map[string]domain.EnvPortScan, len(byDir))
	for dir, sources := range byDir {
		sort.SliceStable(sources, func(i, j int) bool {
			return sourceRank(sources[i].File) < sourceRank(sources[j].File)
		})
		scans[dir] = rules.MergeEnvPortSources(rules.MergeEnvPortSourcesParams{Dir: dir, Sources: sources})
	}
	for dir, reason := range failures {
		if _, found := scans[dir]; found {
			continue
		}
		scans[dir] = domain.EnvPortScan{Dir: dir, Err: reason}
	}

	return scans
}

// sourceFilesFor lists the files one classified entry contributes. A .env.local
// entry is its own target; a value entry brings its committed template along.
func sourceFilesFor(file domain.EnvFile) []string {
	if file.Local {
		return []string{file.Target}
	}
	if file.Template == "" {
		return []string{file.Target}
	}
	return []string{file.Target, file.Template}
}

// sourceRank orders a directory's files weakest-first, as MergeEnvPortSources
// expects: template, then .env, then .env.local.
func sourceRank(path string) int {
	switch name := filepath.Base(path); {
	case rules.IsLocalEnvFile(name):
		return 2
	case name == domain.EnvFileName:
		return 1
	default:
		return 0
	}
}

// readEnvPorts returns no error for a file that is simply not there: a lone
// template yields a target that was never provisioned, which is not a failure.
func readEnvPorts(path string) (map[string]int, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		// The reason alone: the report already names the file, relative to the
		// project rather than as the absolute path the syscall echoes back.
		var pathErr *fs.PathError
		if errors.As(err, &pathErr) {
			return nil, pathErr.Err
		}
		return nil, err
	}
	return rules.ExtractEnvPorts(rules.ParseEnv(string(data))), nil
}
