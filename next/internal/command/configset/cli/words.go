package cli

import (
	"github.com/LucasPcq/wtm/next/internal/command/configset"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// words are how the command line words each field of config set's form: the settings as init words them, then the remembered answers.
func words(q configset.Questions, field domain.Field) form.Words {
	path := field.Spec().Path
	switch path {
	case configset.PathRemembered:
		return form.Words{Flag: "--remembered", Label: "Remembered answers", Help: "The answers create remembers, each question=value; the list replaces config.toml's"}
	case configset.PathReplaces:
		return form.Words{Flag: "--replaces", Label: "Made on", Help: "The digest of the config.toml the change was made on (config show's): refused when the file changed since"}
	}
	said, _ := form.Settings{EnvFiles: q.EnvFiles.Options, OnCreate: q.OnCreate.Found, OnClean: q.OnClean.Found}.Words(path)
	return said
}
