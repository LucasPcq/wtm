package widget

import "errors"

func picks() []Pick {
	return []Pick{
		{Label: "a.txt", Value: "a.txt", Tag: "modified", Tone: ToneWarning},
		{Label: "b.txt", Value: "b.txt", Tag: "untracked"},
		{Label: "notes.md", Value: "notes.md"},
	}
}

func refusingBad(value string) error {
	if value == "bad" {
		return errors.New("Not a valid name")
	}
	return nil
}

func branches() []Option {
	return []Option{
		{Label: "main", Value: "main"},
		{Label: "feat/login", Value: "feat/login"},
		{Label: "feat/logout", Value: "feat/logout"},
		{Label: "fix/nav", Value: "fix/nav"},
		{Label: "fix/café", Value: "fix/café"},
	}
}
