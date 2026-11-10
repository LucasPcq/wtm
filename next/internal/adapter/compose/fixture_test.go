package compose

import "github.com/LucasPcq/wtm/next/internal/domain/project"

// read is testdata/docker-compose.yml as Compose reads it, but its digest.
func read() project.Compose {
	return project.Compose{
		File:  "docker-compose.yml",
		Reads: []string{"MAILPIT_PORT", "PG_PORT"},
		Services: []project.ComposeService{
			{Name: "postgres", Image: "postgres:16", Ports: []project.ComposePort{{Host: "5432", Container: "5432", At: project.ComposeSpan{Line: 8, Column: 9, Text: `"5432:5432"`}}}},
			{Name: "redis", Image: "redis:7", Ports: []project.ComposePort{{IP: "127.0.0.1", Host: "6379", Container: "6379", At: project.ComposeSpan{Line: 12, Column: 9, Text: "127.0.0.1:6379:6379"}}}},
			{Name: "mailpit", Image: "axllent/mailpit", Ports: []project.ComposePort{
				{Host: "${MAILPIT_PORT:-8025}", Container: "8025", At: project.ComposeSpan{Line: 16, Column: 9, Text: `"${MAILPIT_PORT:-8025}:8025"`}},
				{Host: "1025", Container: "1025", At: project.ComposeSpan{Line: 18, Column: 20, Text: "1025"}},
			}},
			{Name: "api", Build: true, Ports: []project.ComposePort{{Unreadable: "anchor or alias"}}},
		},
		Names: []project.ComposeName{
			{Kind: project.ComposeContainer, Owner: "postgres", Value: "acme-postgres", At: project.ComposeSpan{Line: 6, Column: 21, Text: "acme-postgres"}},
			{Kind: project.ComposeVolume, Owner: "data", Value: "acme-data", At: project.ComposeSpan{Line: 26, Column: 11, Text: "acme-data"}},
		},
	}
}
