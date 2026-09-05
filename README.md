<p align="center">
  <source media="(prefers-color-scheme: dark)" srcset="images/book-dark.png">
  <source media="(prefers-color-scheme: light)" srcset="images/book-light.png">
  <img alt="Project Logo" src="images/book-dark.png" width="128">
</p>

# book

[![Go Version](https://img.shields.io/github/go-mod/go-version/polymorcodeus/book)](https://go.dev/) [![License](https://img.shields.io/github/license/polymorcodeus/book)](./LICENSE) [![Build Status](https://img.shields.io/github/actions/workflow/status/polymorcodeus/book/ci.yml?branch=main)](https://github.com/polymorcodeus/book/actions)


**TUI bookmark manager for your terminal.**

Terminal bookmark manager with hierarchical organization (Shelf → Collection → Mark). Bookmarks are persisted as plain TOML, enabling version control, clean diffs, and [dotfile manager](https://github.com/polymorcodeus/lnk) integration. Supports both interactive TUI and non-interactive CLI modes for scripting.

Bookmarks ship with stable identifiers (`catalog_id`), full-text search (`book mark search`), atomic writes, schema migration (`book migrate`), and soft-delete recovery (`book mark restore`, `book gc`).

## Quick Demo

```bash
book shelf add --name dev                     # add a shelf non-interactively
book collection add --shelf dev --name docs   # add a collection to a shelf
book mark add https://example.com             # add a bookmark (fetches title)
book mark add https://example.com --shelf dev --collection tools --tags go,cli
book shelf list                               # one name per line; pipe into fzf
book mark list --shelf dev --collection tools --format json
```

## Getting Started

### Install

Build from source:

```bash
go install gitlab.com/polymorcodeus/book@latest
```

Or clone and build:

```bash
git clone https://github.com/polymorcodeus/book.git
cd book
go build .
```

*Requirements:*

- Go 1.26.4+
- macOS or Linux (Windows support is limited — `book mark get` uses `open`/`xdg-open`)
- `XDG_CONFIG_HOME` should be set (used for default config and shelf paths)

### Quick Start

1. **Run the CLI interactive mode** for the full TUI experience:

   ```bash
   book --interactive shelf add
   book --interactive collection add
   book --interactive mark add https://example.com
   ```

2. **Manage shelves and collections non-interactively:**

   ```bash
   book shelf add --name dev --description "software bookmarks"
   book collection add --shelf dev --name docs
   book shelf remove --name dev --confirm
   book collection remove --shelf dev --name docs --confirm
   ```

3. **Add, edit, get, and remove bookmarks non-interactively:**

   ```bash
   book mark add \
     https://go.dev/doc/effective_go \
     --shelf dev \
     --collection docs \
     --title "Effective Go" \
     --tags go,best-practices

   book mark get --id <catalog_id>
   book mark edit --id <catalog_id> --tags go,best-practices
   book mark remove --id <catalog_id> --confirm
   ```

4. **List your bookmarks:**

   Plain-text defaults are pipe-friendly and require no flags:

   ```bash
   book shelf list
   book collection list --shelf dev
   book mark list --shelf dev --collection docs
   ```

   Add `--format json` or `--format toml` for structured output:

   ```bash
   book shelf list --format json
   book collection list --shelf dev --format json
   book mark list --shelf dev --collection docs --format json
   book mark get --id <catalog_id> --format json
   ```

## How It Works

Bookmarks are stored as plain TOML files — one file per shelf. This makes them human-readable, diff-friendly, and safe to version in git.

```bash
$XDG_CONFIG_HOME/book/
├── config              # global config (TOML)
├── theme.json          # TUI theme customization
├── template.json       # TUI template strings
└── shelf.d/
    ├── dev.toml
    └── reading.toml
```

### TOML Shelf File Format

```toml
schema_version = 2
shelf_id = "a0d6e1c2"
shelf_name = "dev"
shelf_desc = "software development bookmarks"

[Collections.docs]
collection_id = "6d264600"
collection_name = "docs"
collection_desc = "language and framework docs"

  [[Collections.docs.marks]]
  catalog_id = "21f96eef"
  title = "Effective Go"
  url = "https://go.dev/doc/effective_go"
  tags = ["go", "best-practices"]
```

- `catalog_id` is a stable URL hash — duplicates are rejected across the entire catalog.
- Collections are keyed by name inside the `[Collections]` table.
- Marks are inline arrays-of-tables per collection.
- Optional RFC3339 timestamps (`created_at`, `updated_at`, `deleted_at`) track each entity's lifecycle; `deleted_at` marks a soft-deleted mark.
- `mark remove` soft-deletes by setting `deleted_at`; the mark is hidden from `list`/`get`/`search` until `book gc` purges it or `mark restore` brings it back.
- `schema_version` is the on-disk data format version (`2`), independent of the tool's release version (v1.x). `book migrate` upgrades older v1 files in place.

## Commands

| Command | What it does |
| --- | --- |
| `shelf add --name <name>` | Add a new shelf (falls back to TUI when `--interactive`) |
| `shelf list` | List all shelves |
| `shelf remove --name <name> --confirm` | Remove a shelf |
| `collection add --shelf <shelf> --name <name>` | Add a new collection (falls back to TUI when `--interactive`) |
| `collection list --shelf <shelf>` | List collections in a shelf |
| `collection remove --shelf <shelf> --name <name> --confirm` | Remove a collection |
| `mark add <url>` | Add a bookmark (optionally non-interactive) |
| `mark edit --id <id> [--title/--tags/--url]` | Edit an existing bookmark (falls back to TUI when `--interactive`) |
| `mark get --id <id>` | Show a bookmark (falls back to TUI when `--interactive`) |
| `mark list --shelf <shelf> --collection <collection>` | List bookmarks in a collection (`--trash` lists soft-deleted) |
| `mark search <query>` | Full-text search by title, URL, or tags |
| `mark remove --id <id> --confirm` | Soft-delete a bookmark (falls back to TUI when `--interactive`) |
| `mark restore` | Restore a soft-deleted bookmark (`--id`, or `--shelf`/`--collection`/`--url`) |
| `migrate` | Upgrade v1 shelf files to the v2 schema |
| `gc` | Purge soft-deleted marks past the retention window |
| `index rebuild` | Rebuild the derived search index |
| `index sync` | Reconcile the index with shelf changes |
| `doctor` | Detect post-merge duplicates and conflicts (`--fix` auto-merges; alias `sync`) |
| `catalog theme` | Generate `theme.json` with default TUI theme |
| `catalog template` | Generate `template.json` with default TUI templates |
| `catalog config` | Create the config file if missing |

## Global Options

| Option | Default | What it does |
| --- | --- | --- |
| `--interactive` | `false` | Enable TUI mode (forms, spinners, ASCII banner) |
| `--confirm` | `false` | Auto-confirm config/theme/shelf file creation |
| `--config-file <path>` | `$XDG_CONFIG_HOME/book/config` | Config file path |
| `--shelf-dir <path>` | `$XDG_CONFIG_HOME/book/shelf.d` | Shelf files directory |
| `--theme-file <path>` | `$XDG_CONFIG_HOME/book/theme.json` | Theme JSON path |
| `--template-file <path>` | `$XDG_CONFIG_HOME/book/template.json` | Template JSON path |
| `--catalog-format` | `toml` | Shelf file format (only `toml` supported) |
| `--format <fmt>` | — | Opt into structured output for `list` and `mark get` (`json` or `toml`; default is human-readable text) |

## Configuration

Config values resolve in this priority:

1. CLI flags
2. Environment variables (`BOOK_CONFIRM`, `BOOK_CONFIG`, `BOOK_SHELF_DIR`, `BOOK_CATALOG_FORMAT`, `BOOK_THEME`, `BOOK_TEMPLATE`)
3. TOML config file
4. Hardcoded defaults

## Example

See example [config](example/book/config), [shelf](example/book/shelf.d/archive.toml), and [template](example/book/template.json) files.

## Customization

### Theme

Run `book catalog theme` to generate a `theme.json` with default values. Edit colors and styles, then set `theme_file` in your config or use the `--theme-file` flag. All TUI colors and lipgloss styles are driven from this file.

### Templates

Run `book catalog template` to generate a `template.json`. This controls the title strings shown in TUI forms (e.g., the main menu header, list headers). Overlay your own values — unset keys keep their defaults.

## Acknowledgements

Built on [Charm](https://charm.sh/)'s excellent BubbleTea, Huh, and Lipgloss libraries. Uses `gofiglet` for the ASCII banner.

## Development

Business logic lives in `internal/book` as pure, testable functions: URL validation, tag parsing, title resolution, ID generation, and shelf/collection/mark helpers. The `cmd` and `internal/model` packages are thin adapters over this layer.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[MIT](LICENSE)
