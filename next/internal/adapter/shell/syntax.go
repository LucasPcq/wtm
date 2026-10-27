package shell

import (
	"bytes"
	"context"
	"errors"
	"strings"
)

const shellSyntaxFlag = "-n"

// Syntax parses line as /bin/sh would run it, running nothing, and fails with
// the shell's own message when the shell refuses it: the shell that runs the
// line judges it, so the check and the run never disagree.
func Syntax(ctx context.Context, line string) error {
	var out bytes.Buffer
	cmd := Command(ctx, shellBin, shellSyntaxFlag, shellCommandFlag, line)
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	if err == nil {
		return nil
	}

	if said := strings.TrimSpace(out.String()); said != "" {
		return errors.New(said)
	}
	return err
}
