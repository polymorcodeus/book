<p align="center">
  <source media="(prefers-color-scheme: dark)" srcset="images/book-dark.png">
  <source media="(prefers-color-scheme: light)" srcset="images/book-light.png">
  <img alt="Project Logo" src="images/book-dark.png" width="128">
</p>

# book

[![Go Version](https://img.shields.io/github/go-mod/go-version/polymorcodeus/book)](https://go.dev/) [![License](https://img.shields.io/github/license/polymorcodeus/book)](./LICENSE) [![Build Status](https://img.shields.io/github/actions/workflow/status/polymorcodeus/book/ci.yml?branch=main)](https://github.com/polymorcodeus/book/actions)


**TUI bookmark manager for your terminal.**

Terminal bookmark manager with hierarchical organization (Shelf → Collection → Mark). Bookmarks are persisted as plain TOML, enabling version control, clean diffs, and [dotfile manager](https://github.com/polymorcodeus/lnk) integration. Supports both interactive TUI and non-interactive CLI modes for scripting.

v1 roadmap includes search, lazy loading, stable identifiers, and atomic shelf-collection operations.

## Quick Demo

```bash
book shelf add                                # add a shelf interactively
book collection add                           # add a collection to a shelf
book mark add https://example.com             # add a bookmark (fetches title)
book mark add https://example.com --shelf dev --collection tools --tags go,cli
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

2. **Add a bookmark non-interactively:**

   ```bash
   book mark add \
     https://go.dev/doc/effective_go \
     --shelf dev \
     --collection docs \
     --title "Effective Go" \
     --tags go,best-practices
   ```

3. **List your bookmarks:**

   ```bash
   book shelf list --format json
   book collection list --shelf dev --format json
   book mark list --shelf dev --collection docs --format json
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
shelf_name = "dev"
shelf_desc = "software development bookmarks"

[Collections.docs]
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

## Commands

| Command | What it does |
| --- | --- |
| `shelf add` | Add a new shelf |
| `shelf list` | List all shelves |
| `shelf remove` | Remove a shelf |
| `collection add` | Add a new collection |
| `collection list` | List collections in a shelf |
| `collection remove` | Remove a collection |
| `mark add <url>` | Add a bookmark (optionally non-interactive) |
| `mark edit` | Edit an existing bookmark (TUI) |
| `mark get` | Browse bookmarks and open one (TUI) |
| `mark list` | List bookmarks in a collection |
| `mark remove` | Remove a bookmark (TUI) |
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
| `--format <fmt>` | — | Output format for `list` commands (`json` or `toml`) |

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

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[MIT](LICENSE)
