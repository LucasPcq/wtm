package hooks

import (
	"bytes"
	"context"
	"strings"
	"sync"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

type ExecuteParams struct {
	Hooks   []domain.HookCommand
	WorkDir string
	Vars    rules.TemplateVars
	// Env nil leaves the hook the environment this process inherited.
	Env map[string]string
	// Output gets each line the hooks print, stdout and stderr alike.
	Output func(line string)
	// OnHook gets each hook's end with its error even when continue_on_error goes past it.
	OnHook func(domain.HookBeat)
}

func Execute(ctx context.Context, params ExecuteParams) error {
	sink := &lines{emit: params.Output}
	return RunHooks(ctx, RunHooksParams{
		Hooks:   params.Hooks,
		WorkDir: params.WorkDir,
		Vars:    params.Vars,
		Env:     params.Env,
		Output:  sink,
		OnHook: func(beat domain.HookBeat) {
			if !beat.Started {
				sink.flush()
			}
			if params.OnHook != nil {
				params.OnHook(beat)
			}
		},
	})
}

// lines flushes a hook's unfinished last line when it ends, never joined to the next hook's first.
type lines struct {
	mu   sync.Mutex
	emit func(string)
	buf  bytes.Buffer
}

func (l *lines) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.buf.Write(p)
	for {
		line, err := l.buf.ReadString('\n')
		if err != nil {
			l.buf.Reset()
			l.buf.WriteString(line)
			return len(p), nil
		}
		l.send(strings.TrimSuffix(line, "\n"))
	}
}

func (l *lines) flush() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.buf.Len() == 0 {
		return
	}
	rest := l.buf.String()
	l.buf.Reset()
	l.send(rest)
}

func (l *lines) send(line string) {
	if l.emit != nil {
		l.emit(line)
	}
}
