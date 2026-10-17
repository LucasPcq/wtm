package store

// The variables that turn the test binary into a helper process updating a
// record: the state directory, its job and how many rounds it runs.
const (
	envStateDir = "WTM_TEST_STORE_STATE_DIR"
	envJob      = "WTM_TEST_STORE_JOB"
	envRounds   = "WTM_TEST_STORE_ROUNDS"
)

// userEnv is a .env file as a user writes it: comments, a blank line, an
// export and a CRLF line ending, all of which an edit must keep.
const userEnv = "# database\nDB_HOST=localhost\nWEB_PORT=3000 # the dev server\n\nexport API_KEY=secret\r\n"

// realRecord is a meta.json as wtm writes it.
const realRecord = "{\n  \"source_branch\": \"0.30.0\",\n  \"created_at\": \"2026-10-16T20:37:05Z\",\n  \"env_strategy\": \"parent\",\n  \"isolation\": \"isolated\"\n}"
