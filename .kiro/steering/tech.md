# Tech Stack

## Language & Runtime

- **Go 1.25+** (module requires `go 1.25.0`, toolchain `go1.25.1`)
- Static binary only: `CGO_ENABLED=0`, `-extldflags -static`, `-tags static_build`
- Linux-only targets: `linux/amd64`, `linux/arm64`

## Key Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/urfave/cli v1.22.9` | CLI framework (migration target: v3) |
| `github.com/sirupsen/logrus` | Structured logging |
| `github.com/ghodss/yaml` | YAML parsing |
| `github.com/rancher/mapper` | Schema-based config type coercion |
| `github.com/moby/sys/reexec` | Multi-personality binary dispatch |
| `github.com/stretchr/testify` | Test assertions and mocking |
| `github.com/pkg/errors` | Error wrapping (migration target: `fmt.Errorf` + `%w`) |
| `pault.ag/go/modprobe` | Kernel module loading |
| `internal/loopdev` (uses `golang.org/x/sys/unix`) | Loop device management |
| `golang.org/x/sys` | Linux syscalls |

## Build & Release

- **GoReleaser v2** — produces static binaries for `linux/amd64` and `linux/arm64`, archives as `.tar.gz`
- Version injected at build time: `-X github.com/petercb/k3os-bin/internal/version.Version={{.Version}}`
- **CircleCI** — CI/CD pipeline; semantic release on `master` via `go-semantic-release` + GoReleaser

## Linting & Formatting

- **golangci-lint v2** with `gofumpt` + `goimports` formatters
- Enabled linters: `bodyclose`, `gocognit`, `gocritic`, `govet` (+ shadow), `misspell`, `revive`, `testifylint`, `unconvert`, `unparam`, `unused`
- Pre-commit hooks via `.pre-commit-config.yaml`

## Common Commands

> Most commands require Linux. On macOS, use Docker (see below).

### Build

```bash
# Development build
GOOS=linux go build -o k3os .

# Production build (matches GoReleaser)
GOOS=linux CGO_ENABLED=0 go build \
  -ldflags "-s -w -extldflags -static -X github.com/petercb/k3os-bin/internal/version.Version=dev" \
  -tags static_build -o k3os .
```

### Test

```bash
# Run all tests (Linux)
go test ./...

# Run with race detector (CI mode)
go test -race -covermode=atomic -failfast ./...

# Run a specific package
go test ./internal/config/...

# Coverage report
go test -coverprofile=coverage.out ./internal/...
go tool cover -func=coverage.out
```

### Test via Docker (macOS)

```bash
# Full test suite
docker run --rm -v "$(pwd)":/app -w /app golang:1 \
  go test -race -covermode=atomic -failfast ./...

# Specific package
docker run --rm -v "$(pwd)":/app -w /app golang:1 \
  go test -v ./internal/config/...
```

### Lint

```bash
# With auto-fix
golangci-lint run --fix ./...

# CI mode (no fix)
golangci-lint run ./...
```

### Validate Config/Docs

```bash
# Markdown
markdownlint-cli2 "**/*.md" "#node_modules"

# YAML
yamlfmt .
yamllint .

# CircleCI config
circleci config validate
```

## Commit Convention

Follows [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<optional scope>): <short description>
```

Types: `feat`, `fix`, `test`, `docs`, `chore`, `refactor`

Branching: single `master` branch. Feature branches: `feature/<task-id>-<short-slug>`.
