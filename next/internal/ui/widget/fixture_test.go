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
