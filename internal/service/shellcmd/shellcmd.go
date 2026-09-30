// Package shellcmd checks that a command written in a config file is a shell
// line the shell can actually parse.
package shellcmd

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// CheckSyntax parses line without running it. The check is delegated to the
// shell that will run it rather than reimplemented, so the verdict here and the
// behaviour at startup can never diverge.
func CheckSyntax(line string) error {
	out, err := exec.Command(domain.ShellBin, domain.ShellSyntaxCheckFlag, domain.ShellCommandFlag, line).CombinedOutput()
	if err == nil {
		return nil
	}

	detail := strings.TrimSpace(string(out))
	if detail == "" {
		detail = err.Error()
	}
	return fmt.Errorf("%s", detail)
}
