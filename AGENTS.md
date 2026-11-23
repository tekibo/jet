# Agent guidelines for the jet repo

Guidelines for agents working on this codebase:

- Build: `go build -ldflags="-s -w" -o jet .`.
- Verify: `go test ./...` and `go vet ./...`; keep `gofmt -l` empty.
- When changing the CLI surface (commands, help text, docs): keep `jet --help`, README.md's
  Commands table and docs/agents.md in sync.
- Tests cover the CLI surface: `go test ./agents/ ./doctor/` after touching those packages.
