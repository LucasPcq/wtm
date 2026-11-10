package settings

// questions are the settings init asks over facts for given.
func questions(f Facts, given Given) Questions {
	return NewQuestions(QuestionsParams{Command: "init", Given: given, Facts: f})
}
