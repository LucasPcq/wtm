package store

const (
	envStateDir = "WTM_TEST_STORE_STATE_DIR"
	envJob      = "WTM_TEST_STORE_JOB"
	envRounds   = "WTM_TEST_STORE_ROUNDS"
)

const userEnv = "# database\nDB_HOST=localhost\nWEB_PORT=3000 # the dev server\n\nexport API_KEY=secret\r\n"

const realRecord = "{\n  \"source_branch\": \"0.30.0\",\n  \"created_at\": \"2026-10-16T20:37:05Z\",\n  \"env_strategy\": \"parent\",\n  \"isolation\": \"isolated\"\n}"
