package cli

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func quietTestCmd(t *testing.T, args ...string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "probe", RunE: func(*cobra.Command, []string) error { return nil }}
	cmd.Flags().Bool(domain.FlagQuiet, false, "")
	cmd.Flags().String(domain.FlagOutput, domain.OutputText, "")
	if err := cmd.Flags().Parse(args); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	return cmd
}

func silenced(cmd *cobra.Command) bool {
	return cmd.OutOrStdout() == io.Discard && cmd.ErrOrStderr() == io.Discard
}

const shieldHelperEnv = "WTM_TEST_SHIELD_HELPER"

type interruptHelper struct {
	cmd   *exec.Cmd
	lines *bufio.Scanner
}

func startHelper(t *testing.T, test string) interruptHelper {
	t.Helper()
	helper := exec.Command(os.Args[0], "-test.run=^"+test+"$")
	helper.Env = append(os.Environ(), shieldHelperEnv+"=1")
	stdout, err := helper.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := helper.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = helper.Process.Kill() })
	return interruptHelper{cmd: helper, lines: bufio.NewScanner(stdout)}
}

func (h interruptHelper) expect(t *testing.T, want string) string {
	t.Helper()
	if !h.lines.Scan() {
		t.Fatalf("helper ended before saying %q", want)
	}
	if want != "" && h.lines.Text() != want {
		t.Fatalf("helper said %q, want %q", h.lines.Text(), want)
	}
	return h.lines.Text()
}

func (h interruptHelper) diesOnSIGINT(t *testing.T) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- h.cmd.Wait() }()
	select {
	case err := <-done:
		var exitErr *exec.ExitError
		status, ok := syscall.WaitStatus(0), false
		if errors.As(err, &exitErr) {
			status, ok = exitErr.Sys().(syscall.WaitStatus)
		}
		if !ok || !status.Signaled() || status.Signal() != syscall.SIGINT {
			t.Fatalf("helper ended with %v, want SIGINT", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the second interrupt did not end the run")
	}
}

type exclusivePair struct {
	cmd  *cobra.Command
	args []string
}

func exclusivePairs(cmd *cobra.Command) []exclusivePair {
	var pairs []exclusivePair
	seen := map[string]bool{}
	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		for _, group := range f.Annotations[domain.AnnotationMutuallyExclusive] {
			if seen[group] {
				continue
			}
			seen[group] = true
			names := strings.Fields(group)
			for i, first := range names {
				for _, second := range names[i+1:] {
					path := strings.Fields(cmd.CommandPath())[1:]
					args := append(path, flagArg(cmd, first), flagArg(cmd, second))
					pairs = append(pairs, exclusivePair{cmd: cmd, args: args})
				}
			}
		}
	})
	for _, sub := range cmd.Commands() {
		pairs = append(pairs, exclusivePairs(sub)...)
	}
	return pairs
}

func flagArg(cmd *cobra.Command, name string) string {
	if cmd.Flags().Lookup(name).Value.Type() == "bool" {
		return "--" + name
	}
	return "--" + name + "=x"
}

func resetFlags(cmd *cobra.Command) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		_ = f.Value.Set(f.DefValue)
		f.Changed = false
	})
}

func walk(cmd *cobra.Command, visit func(*cobra.Command)) {
	visit(cmd)
	for _, sub := range cmd.Commands() {
		walk(sub, visit)
	}
}

const interruptHelperEnv = "WTM_TEST_INTERRUPT_HELPER"
