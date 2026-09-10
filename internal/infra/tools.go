package infra

import (
	"os/exec"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// BaseBranch detects the default remote branch for the repo at dir via
// git symbolic-ref. Returns domain.DefaultBaseBranch if detection fails.
func BaseBranch(dir string) string {
	cmd := exec.Command("git", "symbolic-ref", "refs/remotes/origin/HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return domain.DefaultBaseBranch
	}

	ref := strings.TrimSpace(string(out))
	parts := strings.Split(ref, "/")
	if len(parts) == 0 {
		return domain.DefaultBaseBranch
	}

	return parts[len(parts)-1]
}

// DockerComposeCommand returns the docker-compose invocation available on the
// host: "docker compose" (v2 plugin), "docker-compose" (v1 standalone), or
// "docker compose" as a safe default when neither is detectable.
func DockerComposeCommand() string {
	if _, err := exec.LookPath(domain.DockerBin); err == nil {
		if err := exec.Command(domain.DockerBin, domain.ComposeSubcommand, domain.ComposeVersionArg).Run(); err == nil {
			return domain.ComposeCommand
		}
	}
	if _, err := exec.LookPath(domain.ComposeLegacyBin); err == nil {
		return domain.ComposeLegacyBin
	}
	return domain.ComposeCommand
}
