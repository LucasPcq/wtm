package runenv

import "github.com/LucasPcq/wtm/internal/domain"

type LinkKind string

const (
	LinkPort  LinkKind = "env_port"
	LinkValue LinkKind = "env"
)

type Ignored struct {
	Kind LinkKind
	Key  string
	File string
}

type TargetsParams struct {
	Run   domain.RunConfig
	Files []domain.EnvFile
}

func Targets(params TargetsParams) (domain.RunConfig, []Ignored) {
	declared := make(map[string]bool, len(params.Files))
	for _, file := range params.Files {
		declared[file.Target] = true
	}
	kept := params.Run
	kept.EnvPorts, kept.EnvValues = nil, nil
	var ignored []Ignored
	for _, link := range params.Run.EnvPorts {
		if !declared[link.File] {
			ignored = append(ignored, Ignored{Kind: LinkPort, Key: link.Key, File: link.File})
			continue
		}
		kept.EnvPorts = append(kept.EnvPorts, link)
	}
	for _, link := range params.Run.EnvValues {
		if !declared[link.File] {
			ignored = append(ignored, Ignored{Kind: LinkValue, Key: link.Key, File: link.File})
			continue
		}
		kept.EnvValues = append(kept.EnvValues, link)
	}
	return kept, ignored
}
