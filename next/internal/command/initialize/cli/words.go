package cli

import (
	"github.com/LucasPcq/wtm/next/internal/command/initialize"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// words are how the command line words each setting of init's form: the words every command over the settings shares.
func words(q initialize.Questions, field domain.Field) form.Words {
	said, _ := form.Settings{EnvFiles: q.EnvFiles.Options, OnCreate: q.OnCreate.Found, OnClean: q.OnClean.Found}.Words(field.Spec().Path)
	return said
}
