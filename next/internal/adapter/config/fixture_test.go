package config

const fullConfig = `
[worktrees]
base_path = ".trees"
base_branch = "develop"

[env]
strategy = "main"

[[env.file]]
target = ".env"

[[env.file]]
target = "apps/api/.env"

[hooks]
on_create = [
  "pnpm install",
  { cmd = "pnpm install", cwd = "apps/api" },
]
`

const minimalConfig = `
[worktrees]
`

const newerSchema = "schema = 99\n"

const awkward = "say \"hi\" \\ C:\\tmp 'q'\nnext\ttab \x7f"

const awkwardConfig = `
[worktrees]
base_path = "../say \"hi\" \\ C:\\tmp 'q'\nnext\ttab \u007f"
base_branch = "feat/say \"hi\" \\ C:\\tmp 'q'\nnext\ttab \u007f"

[env]
strategy = "example"

[[env.file]]
target = "say \"hi\" \\ C:\\tmp 'q'\nnext\ttab \u007f/.env"
template = "say \"hi\" \\ C:\\tmp 'q'\nnext\ttab \u007f/.env.example"

[hooks]
on_create = [
  "say \"hi\" \\ C:\\tmp 'q'\nnext\ttab \u007f",
  { cmd = "say \"hi\" \\ C:\\tmp 'q'\nnext\ttab \u007f", cwd = "say \"hi\" \\ C:\\tmp 'q'\nnext\ttab \u007f", continue_on_error = true },
]
on_clean = [
  "say \"hi\" \\ C:\\tmp 'q'\nnext\ttab \u007f",
]
`
