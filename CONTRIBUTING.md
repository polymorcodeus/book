# Contributing to polybook

Thanks for considering a contribution! polybook is a terminal-native bookmark manager — keeping the CLI fast, the TUI pleasant, and the TOML storage predictable is the top priority.

## Quick Start

```bash
git clone <your-fork>
cd polybook
go build ./...
go test ./...
go vet ./...
```

## Project Structure

```sh
cmd/book/
├── main.go          # CLI command tree (urfave/cli/v3), flags, Before hook
├── shelf.go         # shelf command actions + TUI root screen
├── collection.go    # collection command actions + TUI root screen
└── mark.go          # mark command actions (add, get, edit, remove) + TUI root screen

internal/book/
└── types.go         # Core data structs: Config, BookShelves, Shelf, Collection, Mark

internal/catalog/
├── catalog.go       # VerifyExists, LoadShelves, LoadCatalog (spinner)
└── toml.go          # TOML read/write, atomic writes, config creation

internal/web/
└── web.go           # OpenURL, WebsiteTitle, LoadWebsite

internal/theme/
└── theme.go         # Theme loading from JSON, color/style resolution, huh theme generation

internal/model/
├── tea.go           # Shared TUI types: Styles, Book, RootScreen, errMsg
├── shelf_model.go   # getShelfModel, editShelfModel
├── collection_model.go  # getCollectionModel, editCollectionModel
└── mark_model.go    # getMarkModel, editMarkModel
```

| Package | Imports | Does NOT import |
|---------|---------|-----------------|
| `internal/book` | stdlib + `theme` | `internal/catalog`, `internal/model` |
| `internal/theme` | `huh`, `lipgloss`, `json` | `internal/book`, `internal/catalog`, `internal/model` |
| `internal/web` | `book`, `goquery`, `huh`, `lipgloss`, `spinner` | `internal/catalog`, `internal/model` |
| `internal/catalog` | `book`, `toml`, `huh`, `lipgloss`, `spinner`, `theme` | `internal/model` |
| `internal/model` | `book`, `catalog`, `theme`, `web`, `huh`, `lipgloss`, `bubbletea` | — |
| `cmd` | everything | — |

## Conventions

### Go Version

Target **Go 1.26.4**. Avoid language features you aren't certain exist in this version. When in doubt, check [go.dev/doc/go1.26](https://go.dev/doc/go1.26).

### BubbleTea Architecture

All blocking I/O must happen inside `tea.Cmd` closures, not in `Update()`. File writes, URL opens, and network calls are wrapped in command functions that return messages. This is non-negotiable — the TUI hangs otherwise.

- Use `updateShelfFileCmd(action)` pattern in all `edit*Model` implementations.
- Batch `tea.Quit` with file commands when saving.
- Never create `lipgloss.NewStyle()` inside `View()`. Use pre-defined styles from `Book.styles`.

### Error Handling

Return errors rather than silently falling back. The codebase favors colloquial error messages (e.g., "betta check yerself") — keep the tone, but always propagate the error to the caller. `StyledError` renders a styled banner in interactive mode; plain text in non-interactive mode.

### TOML Tags

Use the established tag convention:

- `shelf_name`, `shelf_desc`
- `collection_name`, `collection_desc`
- `title`, `url`, `tags`
- `catalog_id`

### Configuration Layering

Config resolution order is fixed: **CLI flags > env vars > TOML config > defaults**. Use `cli.NewValueSourceChain` when wiring new flags. If you add a flag, consider whether it should also be readable from env and config file.

### Theming

All styling goes through `internal/theme/theme.go`. Never hardcode colors or lipgloss styles outside of the theme package. The `Styles` struct in `internal/model/tea.go` is the single source of truth for TUI rendering.

## Code Quality

Run the full check before pushing:

```bash
make check          # fmt, vet, lint, test
```

Or manually:

```bash
go fmt ./...
go vet ./...
golangci-lint run   # create .golangci.yml if you want custom config
go test ./...
```

### Linting

We use `golangci-lint` via `make lint`. Key linters to care about: `errcheck` (explicit `Close()` handling), `govet`, `ineffassign`, `staticcheck`, `unused`.

Note: there is no `.golangci.yml` in the repo yet. If you want to add one, open an issue first.

### Tests

There are currently **zero tests**. New features or bug fixes **should** include tests where feasible. Priority targets for coverage:

- `VerifyUniqueURL`, `DedupUnique`, `MergeTags`, `StructIsEmpty`, `GenerateID` in `internal/book`
- TOML round-trip encoding/decoding in `internal/catalog`
- `WebsiteTitle` / `OpenURL` in `internal/web` (mock HTTP server)

Prefer table-driven tests. The TUI layer (`internal/model`) is harder to unit test — focus on extracting pure logic into `internal/book` instead.

## Pull Request Process

1. **Open an issue first** for significant changes (new commands, breaking changes, new dependencies, storage format changes).
2. **Fork and branch**: `git checkout -b fix/description` or `feature/description`.
3. **Write tests** for any new pure-logic behavior or bug fix.
4. **Run `make check`** and ensure everything passes.
5. **Update docs** if you change commands, flags, or config behavior.
6. **Reference the issue** in your PR description.

## Questions?

Open an issue. For bug reports, include:

- Go version (`go version`)
- Command that triggers the issue
- Expected vs actual behavior
- Whether `--interactive` is involved
