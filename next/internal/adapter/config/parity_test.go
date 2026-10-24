package config

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheProjectAndItsFakeLoadTheConfigTheyHold(t *testing.T) {
	for _, impl := range implementations(t, seed{Config: seeded()}) {
		t.Run(impl.name, func(t *testing.T) {
			config, global, err := impl.port.Load()

			require.NoError(t, err)
			assert.Equal(t, *seeded(), config)
			assert.Equal(t, project.Global{}, global)
		})
	}
}

func TestTheProjectAndItsFakeWithoutAConfigAreNotFound(t *testing.T) {
	for _, impl := range implementations(t, seed{}) {
		t.Run(impl.name, func(t *testing.T) {
			_, _, loadErr := impl.port.Load()
			_, rememberErr := impl.port.Remember(t.Context(), map[string]string{project.RememberIsolation: "verbatim"})

			require.ErrorIs(t, loadErr, project.ErrConfigNotFound)
			require.ErrorIs(t, rememberErr, project.ErrConfigNotFound)
		})
	}
}

func TestTheProjectAndItsFakeRefuseAConfigThatDoesNotParse(t *testing.T) {
	for _, impl := range implementations(t, seed{Config: seeded(), Unreadable: true, Schema: 99}) {
		t.Run(impl.name, func(t *testing.T) {
			_, _, loadErr := impl.port.Load()
			_, rememberErr := impl.port.Remember(t.Context(), map[string]string{project.RememberIsolation: "verbatim"})

			_, invalid := errors.AsType[*project.InvalidConfigError](loadErr)
			assert.True(t, invalid, "Load = %v, want a *project.InvalidConfigError", loadErr)
			require.Error(t, rememberErr)
			_, newer := errors.AsType[*domain.SchemaNewer](rememberErr)
			assert.False(t, newer, "a file that does not parse reads no schema: Remember = %v", rememberErr)
		})
	}
}

func TestTheProjectAndItsFakeRememberOnTopOfWhatTheyHoldAndNeverForget(t *testing.T) {
	for _, impl := range implementations(t, seed{Config: seeded()}) {
		t.Run(impl.name, func(t *testing.T) {
			written, err := impl.port.Remember(t.Context(), map[string]string{project.RememberIsolation: "verbatim"})
			again, againErr := impl.port.Remember(t.Context(), map[string]string{project.RememberIsolation: "verbatim"})

			require.NoError(t, err)
			require.NoError(t, againErr)
			assert.Equal(t, domain.Written{Changed: true}, written)
			assert.Equal(t, domain.Written{}, again)
			config, _, err := impl.port.Load()
			require.NoError(t, err)
			assert.Equal(t, map[string]string{project.RememberEnvStrategy: "main", project.RememberIsolation: "verbatim"}, config.Remembered)
		})
	}
}

func TestTheProjectAndItsFakeRememberNothingWhenNoAnswerIsGiven(t *testing.T) {
	for _, impl := range implementations(t, seed{}) {
		t.Run(impl.name, func(t *testing.T) {
			written, err := impl.port.Remember(t.Context(), nil)

			require.NoError(t, err)
			assert.Equal(t, domain.Written{}, written)
		})
	}
}

func TestTheProjectAndItsFakeNeverRewriteAConfigAtANewerSchema(t *testing.T) {
	for _, impl := range implementations(t, seed{Config: seeded(), Schema: 99}) {
		t.Run(impl.name, func(t *testing.T) {
			_, err := impl.port.Remember(t.Context(), map[string]string{project.RememberIsolation: "verbatim"})

			var refusal *domain.SchemaNewer
			require.ErrorAs(t, err, &refusal)
			config, _, err := impl.port.Load()
			require.NoError(t, err)
			assert.Equal(t, seeded().Remembered, config.Remembered)
		})
	}
}

func TestTheProjectAndItsFakeReadNoRunTomlAsAnEmptyRunConfig(t *testing.T) {
	for _, impl := range implementations(t, seed{Config: seeded()}) {
		t.Run(impl.name, func(t *testing.T) {
			run, err := impl.port.LoadRun()

			require.NoError(t, err)
			assert.Equal(t, project.Run{}, run)
		})
	}
}
