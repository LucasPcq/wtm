// Package shellcmd checks that a command written in a config file is a shell
// line the shell can actually parse.
package shellcmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
)

// CheckSyntax parses line without running it. The check is delegated to the
// shell that will run it rather than reimplemented, so the verdict here and the
// behaviour at startup can never diverge.
func CheckSyntax(ctx context.Context, line string) error {
	out, err := infra.Command(ctx, domain.ShellBin, domain.ShellSyntaxCheckFlag, domain.ShellCommandFlag, line).CombinedOutput()
	if err == nil {
		return nil
	}

	detail := strings.TrimSpace(string(out))
	if detail == "" {
		detail = err.Error()
	}
	return fmt.Errorf("%s", detail)
}
