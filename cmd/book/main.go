// Package cmd implements the book CLI.
package cmd

import (
	"context"
	"fmt"
	"image/color"
	"log"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/polymorcodeus/gofiglet"
	altsrc "github.com/urfave/cli-altsrc/v3"
	alttoml "github.com/urfave/cli-altsrc/v3/toml"
	validation "github.com/urfave/cli-validation"
	"github.com/urfave/cli/v3"

	"github.com/polymorcodeus/book/internal/book"
	"github.com/polymorcodeus/book/internal/catalog"
	"github.com/polymorcodeus/book/internal/model"
)

var (
	version   = "internal"
	buildTime string
)

// SetVersion sets the application version string used by the CLI.
func SetVersion(v string) {
	version = v
}

// SetBuildTime sets the build timestamp for version display.
func SetBuildTime(bt string) {
	buildTime = bt
}

func buildVersion() string {
	v := version
	if buildTime != "" {
		v += " (" + buildTime + ")"
	}
	return v
}

// Main builds and runs the book CLI application.
func Main() {
	cache := &indexCache{}
	defer func() { _ = cache.close() }()

	var confirm bool
	var interactive bool
	var format string

	var config *book.Config
	var configFile string
	var themeFile string
	var templateFile string
	var shelfDir string
	var catalogFormat string

	var bookShelves book.BookShelves

	var shelf string
	var collection string
	var shelfName string
	var shelfDescription string
	var collectionName string
	var collectionDescription string

	var markURL string
	var markTags string
	var markTitle string
	var searchTags string
	var restoreURL string
	var restoreID string
	var markID string
	var trash bool
	var retentionDays int
	var fix bool

	cmd := &cli.Command{
		Name:                  "book",
		Usage:                 "mark ur life!",
		Description:           "all your marks are belong to us ...",
		Version:               buildVersion(),
		EnableShellCompletion: true,
		HideVersion:           false,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:        "confirm",
				Value:       false,
				Usage:       "set to confirm config, theme, and shelf file updates",
				Destination: &confirm,
				Sources: cli.NewValueSourceChain(
					cli.EnvVar("BOOK_CONFIRM"),
					alttoml.TOML("autoconfirm", altsrc.NewStringPtrSourcer(&configFile)),
				),
			},
			&cli.StringFlag{
				Name:        "config-file",
				Value:       filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "book", "config"),
				Usage:       "path to book config file",
				Destination: &configFile,
				Sources:     cli.EnvVars("BOOK_CONFIG"),
			},
			&cli.StringFlag{
				Name:        "shelf-dir",
				Value:       filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "book", "shelf.d"),
				Usage:       "directory of book shelves",
				Destination: &shelfDir,
				Sources: cli.NewValueSourceChain(
					cli.EnvVar("BOOK_SHELF_DIR"),
					alttoml.TOML("shelf_directory", altsrc.NewStringPtrSourcer(&configFile)),
				),
			},
			&cli.StringFlag{
				Name:        "catalog-format",
				Value:       "toml",
				Usage:       "format of book shelves - only toml support currently",
				Destination: &catalogFormat,
				Sources: cli.NewValueSourceChain(
					cli.EnvVar("BOOK_CATALOG_FORMAT"),
					alttoml.TOML("catalog_format", altsrc.NewStringPtrSourcer(&configFile)),
				),
				Validator: validation.Enum("toml"),
			},
			&cli.StringFlag{
				Name:        "theme-file",
				Value:       filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "book", "theme.json"),
				Usage:       "path to book theme file",
				Destination: &themeFile,
				Sources: cli.NewValueSourceChain(
					cli.EnvVar("BOOK_THEME"),
					alttoml.TOML("theme_file", altsrc.NewStringPtrSourcer(&configFile)),
				),
			},
			&cli.StringFlag{
				Name:        "template-file",
				Value:       filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "book", "template.json"),
				Usage:       "path to book template file",
				Destination: &templateFile,
				Sources: cli.NewValueSourceChain(
					cli.EnvVar("BOOK_TEMPLATE"),
					alttoml.TOML("template_file", altsrc.NewStringPtrSourcer(&configFile)),
				),
			},
			&cli.StringFlag{
				Name:        "format",
				Usage:       "output format of non-interactive capable commands, e.g. `book mark list`",
				Destination: &format,
				Validator:   validation.Enum("toml", "json"),
			},
			&cli.BoolFlag{
				Name:        "interactive",
				Value:       false,
				Usage:       "set to true to enable TUI and other visuals",
				Destination: &interactive,
				Sources: cli.NewValueSourceChain(
					alttoml.TOML("interactive", altsrc.NewStringPtrSourcer(&configFile)),
				),
			},
		},
		Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			config = &book.Config{
				ConfigFile:    configFile,
				ThemeFile:     themeFile,
				TemplateFile:  templateFile,
				CatalogFormat: catalogFormat,
				ShelfRoot:     shelfDir,
				Autoconfirm:   confirm,
				Interactive:   interactive,
			}

			// Load theme-file (uses built-in defaults if file doesn't exist), use Huhbase as fallback
			if err := config.LoadTheme(interactive); err != nil {
				return ctx, cli.Exit(err, 1)
			}

			if config.Interactive {
				if err := config.LoadTemplates(); err != nil {
					return ctx, cli.Exit(err, 1)
				}
			}

			// Load Book Shelves only if <command> <subcommand> is passed.
			// Additionally, this is skipped for `mark search` (which reads the
			// SQLite index) and the catalog admin tools. This is intentional:
			// when only a subcommand is given (e.g. "book shelf"), urfave/cli
			// will auto-render the help text. We skip catalog loading so help
			// renders quickly without reading the filesystem.
			// Load Book Shelves only for the data commands (shelf, collection,
			// mark) and only when a subcommand is given. This skips `mark
			// search` (which reads the SQLite index) and the catalog admin
			// tools (migrate, gc, doctor, index, catalog), which load their
			// own data. When only a subcommand is given (e.g. "book shelf"),
			// urfave/cli auto-renders help, so we skip loading to keep help
			// fast. Note the command name must be checked explicitly: a
			// top-level tool's own flags (e.g. "doctor --fix") would otherwise
			// leak into Args() and trigger an unwanted load.
			isSearch := cmd.Args().First() == "mark" && cmd.Args().Get(1) == "search"
			needsCatalog := cmd.Args().First() == "shelf" ||
				cmd.Args().First() == "collection" ||
				cmd.Args().First() == "mark"
			if cmd.Args().Len() > 1 && needsCatalog && !isSearch {
				if err := catalog.LoadCatalog(&bookShelves, config, config.Interactive); err != nil {
					return ctx, cli.Exit(config.StyledError(err), 1)
				}
			}

			if config.Interactive {
				bannerRoot := cmd.Root().Name
				bannerCmnd := cmd.Args().First()
				bookBanner, err := gofiglet.NewCmdBanner(
					[]string{bannerRoot, bannerCmnd},
					gofiglet.WithZeroPadding(),
					gofiglet.WithColors([]color.Color{config.Theme.Color("primary_accent"), config.Theme.Color("secondary_accent")}),
				)
				if err != nil {
					return ctx, cli.Exit(config.StyledError(err), 1)
				}
				if _, err = gofiglet.PrintCmdBanner(bookBanner); err != nil {
					return ctx, cli.Exit(config.StyledError(err), 1)
				}
			}
			return ctx, nil
		},
		Commands: []*cli.Command{
			{
				Name:  "shelf",
				Usage: "options for shelves",
				Commands: []*cli.Command{
					{
						Name:  "add",
						Usage: "add a new shelf",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:        "name",
								Usage:       "name of the new shelf",
								Destination: &shelfName,
							},
							&cli.StringFlag{
								Name:        "description",
								Usage:       "description of the new shelf",
								Destination: &shelfDescription,
							},
						},
						Action: func(ctx context.Context, cmd *cli.Command) error {
							if err := addShelf(&bookShelves, shelfName, shelfDescription, config); err != nil {
								return cli.Exit(config.StyledError(err), 1)
							}
							return nil
						},
					},
					{
						Name:    "list",
						Usage:   "list shelves",
						Aliases: []string{"ls"},
						Action: func(ctx context.Context, cmd *cli.Command) error {
							if err := shelves(cache, &bookShelves, format, config); err != nil {
								return cli.Exit(config.StyledError(err), 1)
							}
							return nil
						},
					},
					{
						Name:    "remove",
						Usage:   "remove an existing shelf",
						Aliases: []string{"rm"},
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:        "name",
								Usage:       "name of the shelf to remove",
								Destination: &shelfName,
							},
						},
						Action: func(ctx context.Context, cmd *cli.Command) error {
							if err := removeShelf(&bookShelves, shelfName, confirm); err != nil {
								return cli.Exit(config.StyledError(err), 1)
							}
							return nil
						},
					},
				},
			},
			{
				Name:  "collection",
				Usage: "options for collections",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:        "shelf",
						Usage:       "shelf selection for collection",
						Destination: &shelf,
					},
				},
				Commands: []*cli.Command{
					{
						Name:  "add",
						Usage: "add a new collection to persona",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:        "name",
								Usage:       "name of the new collection",
								Destination: &collectionName,
							},
							&cli.StringFlag{
								Name:        "description",
								Usage:       "description of the new collection",
								Destination: &collectionDescription,
							},
						},
						Action: func(ctx context.Context, cmd *cli.Command) error {
							if err := addCollection(&bookShelves, shelf, collectionName, collectionDescription, config); err != nil {
								return cli.Exit(config.StyledError(err), 1)
							}
							return nil
						},
					},
					{
						Name:    "list",
						Usage:   "list collections in shelve",
						Aliases: []string{"ls"},
						Action: func(ctx context.Context, cmd *cli.Command) error {
							if err := collections(cache, &bookShelves, shelf, format, config); err != nil {
								return cli.Exit(config.StyledError(err), 1)
							}
							return nil
						},
					},
					{
						Name:    "remove",
						Usage:   "remove an existing collection from a persona",
						Aliases: []string{"rm"},
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:        "name",
								Usage:       "name of the collection to remove",
								Destination: &collectionName,
							},
						},
						Action: func(ctx context.Context, cmd *cli.Command) error {
							if err := removeCollection(&bookShelves, shelf, collectionName, confirm); err != nil {
								return cli.Exit(config.StyledError(err), 1)
							}
							return nil
						},
					},
				},
			},
			{
				Name:  "mark",
				Usage: "options for bookmarks",
				Commands: []*cli.Command{
					{
						Name:  "add",
						Usage: "add a new bookmark",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:        "shelf",
								Usage:       "shelf selection for mark",
								Destination: &shelf,
							},
							&cli.StringFlag{
								Name:        "collection",
								Usage:       "collection selection for mark",
								Destination: &collection,
							},
							&cli.StringFlag{
								Name:        "tags",
								Usage:       "comma-separated list of tags to add to mark",
								Destination: &markTags,
							},
							&cli.StringFlag{
								Name:        "url",
								Usage:       "url to define mark",
								Destination: &markURL,
							},
							&cli.StringFlag{
								Name:        "title",
								Usage:       "title of the mark (optional, fetched from URL if not provided)",
								Destination: &markTitle,
							},
						},
						Before: func(ctx context.Context, c *cli.Command) (context.Context, error) {
							if c.Args().First() != "" && markURL != "" {
								return ctx, cli.Exit(config.StyledError(
									fmt.Errorf("cannot pass url as both --url and a positional argument")), 1)
							}

							// use positional argument to populate markURL if flag not used
							if c.Args().First() != "" {
								markURL = c.Args().First()
							}
							if markURL == "" {
								return ctx, cli.Exit(config.StyledError(
									fmt.Errorf("must pass url as --url or argument")), 1)
							}
							return ctx, nil
						},
						Action: func(ctx context.Context, cmd *cli.Command) error {
							if err := addMark(ctx, &bookShelves, markURL, markTags, shelf, collection, markTitle, config); err != nil {
								return cli.Exit(config.StyledError(err), 1)
							}
							return nil
						},
					},
					{
						Name:  "edit",
						Usage: "edit an existing bookmark",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:        "id",
								Usage:       "catalog_id of the mark to edit",
								Destination: &markID,
							},
							&cli.StringFlag{
								Name:        "title",
								Usage:       "new title for the mark",
								Destination: &markTitle,
							},
							&cli.StringFlag{
								Name:        "tags",
								Usage:       "comma-separated tags to replace the mark's tags",
								Destination: &markTags,
							},
							&cli.StringFlag{
								Name:        "url",
								Usage:       "new URL for the mark",
								Destination: &markURL,
							},
						},
						Action: func(ctx context.Context, cmd *cli.Command) error {
							if err := editMark(&bookShelves, markID, markTitle, markTags, markURL, config); err != nil {
								return cli.Exit(config.StyledError(err), 1)
							}
							return nil
						},
					},
					{
						Name:  "get",
						Usage: "browse bookmarks and open selected",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:        "id",
								Usage:       "catalog_id of the mark to open",
								Destination: &markID,
							},
							&cli.StringFlag{
								Name:        "url",
								Usage:       "URL of the mark to open (alternative to --id)",
								Destination: &markURL,
							},
						},
						Action: func(ctx context.Context, cmd *cli.Command) error {
							if err := getMark(&bookShelves, markID, markURL, format, config); err != nil {
								return cli.Exit(config.StyledError(err), 1)
							}
							return nil
						},
					},
					{
						Name:    "list",
						Usage:   "list marks in a collection",
						Aliases: []string{"ls"},
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:        "shelf",
								Usage:       "shelf selection for mark",
								Destination: &shelf,
							},
							&cli.StringFlag{
								Name:        "collection",
								Usage:       "collection selection for mark",
								Destination: &collection,
							},
							&cli.BoolFlag{
								Name:        "trash",
								Usage:       "list soft-deleted marks instead of active ones",
								Destination: &trash,
							},
						},
						Action: func(ctx context.Context, cmd *cli.Command) error {
							if err := marks(cache, &bookShelves, shelf, collection, format, trash, config); err != nil {
								return cli.Exit(config.StyledError(err), 1)
							}
							return nil
						},
					},
					{
						Name:  "search",
						Usage: "search bookmarks by title, URL, or tags",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:        "shelf",
								Usage:       "filter results by shelf",
								Destination: &shelf,
							},
							&cli.StringFlag{
								Name:        "collection",
								Usage:       "filter results by collection",
								Destination: &collection,
							},
							&cli.StringFlag{
								Name:        "tags",
								Usage:       "filter results by tags; comma=OR, plus=AND (e.g. a,b+c)",
								Destination: &searchTags,
							},
						},
						Before: func(ctx context.Context, c *cli.Command) (context.Context, error) {
							if c.Args().First() == "" && searchTags == "" && shelf == "" && collection == "" {
								return ctx, cli.Exit(config.StyledError(fmt.Errorf("must pass a search query or a filter (--tags, --shelf, --collection)")), 1)
							}
							return ctx, nil
						},
						Action: func(ctx context.Context, c *cli.Command) error {
							if err := searchMarks(cache, c.Args().First(), searchTags, shelf, collection, format, config); err != nil {
								return cli.Exit(config.StyledError(err), 1)
							}
							return nil
						},
					},
					{
						Name:    "remove",
						Usage:   "remove an existing bookmark",
						Aliases: []string{"rm"},
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:        "id",
								Usage:       "catalog_id of the mark to remove",
								Destination: &markID,
							},
						},
						Action: func(ctx context.Context, cmd *cli.Command) error {
							if err := removeMark(&bookShelves, markID, confirm, config); err != nil {
								return cli.Exit(config.StyledError(err), 1)
							}
							return nil
						},
					},
					{
						Name:  "restore",
						Usage: "restore a soft-deleted bookmark",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:        "id",
								Usage:       "catalog_id of the trashed mark to restore (preferred)",
								Destination: &restoreID,
							},
							&cli.StringFlag{
								Name:        "shelf",
								Usage:       "shelf containing the trashed mark",
								Destination: &shelf,
							},
							&cli.StringFlag{
								Name:        "collection",
								Usage:       "collection containing the trashed mark",
								Destination: &collection,
							},
							&cli.StringFlag{
								Name:        "url",
								Usage:       "url of the trashed mark to restore",
								Destination: &restoreURL,
							},
						},
						Action: func(ctx context.Context, cmd *cli.Command) error {
							if err := restoreMark(&bookShelves, restoreID, shelf, collection, restoreURL); err != nil {
								return cli.Exit(config.StyledError(err), 1)
							}
							return nil
						},
					},
				},
			},
			{
				Name:  "migrate",
				Usage: "migrate shelf TOML files from v1 to v2 schema",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					if err := migrate(config); err != nil {
						return cli.Exit(config.StyledError(err), 1)
					}
					return nil
				},
			},
			{
				Name:  "gc",
				Usage: "purge soft-deleted marks older than the retention window",
				Flags: []cli.Flag{
					&cli.IntFlag{
						Name:        "retention-days",
						Value:       30,
						Usage:       "purge marks soft-deleted more than this many days ago",
						Destination: &retentionDays,
					},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					if err := gc(cache, config, retentionDays); err != nil {
						return cli.Exit(config.StyledError(err), 1)
					}
					return nil
				},
			},
			{
				Name:    "doctor",
				Aliases: []string{"sync"},
				Usage:   "detect and fix post-merge catalog problems",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:        "fix",
						Usage:       "auto-merge duplicate marks (requires --confirm)",
						Destination: &fix,
					},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					if err := doctor(cache, config, fix); err != nil {
						return cli.Exit(config.StyledError(err), 1)
					}
					return nil
				},
			},
			{
				Name:  "index",
				Usage: "manage the derived SQLite search index",
				Commands: []*cli.Command{
					{
						Name:  "rebuild",
						Usage: "wipe and rebuild the index from shelf TOML files",
						Action: func(ctx context.Context, cmd *cli.Command) error {
							if err := runIndexRebuild(cache, config); err != nil {
								return cli.Exit(config.StyledError(err), 1)
							}
							return nil
						},
					},
					{
						Name:  "sync",
						Usage: "reconcile the index with changes to shelf TOML files",
						Action: func(ctx context.Context, cmd *cli.Command) error {
							if err := runIndexSync(cache, config); err != nil {
								return cli.Exit(config.StyledError(err), 1)
							}
							return nil
						},
					},
				},
			},
			{
				Name:  "catalog",
				Usage: "options for catalog - e.g. admin + customization",
				Commands: []*cli.Command{
					{
						Name:  "theme",
						Usage: "creates theme.json for TUI customization from default values",
						Action: func(ctx context.Context, cmd *cli.Command) error {
							if err := catalog.DumpDefaults(config, "theme"); err != nil {
								return cli.Exit(config.StyledError(err), 1)
							}
							return nil
						},
					},
					{
						Name:  "template",
						Usage: "creates template.json for TUI customization from default values",
						Action: func(ctx context.Context, cmd *cli.Command) error {
							if err := catalog.DumpDefaults(config, "template"); err != nil {
								return cli.Exit(config.StyledError(err), 1)
							}
							return nil
						},
					},
					{
						Name:  "config",
						Usage: "creates config file for greater customizaiton if it doesn't exist",
						Action: func(ctx context.Context, cmd *cli.Command) error {
							// Create config-file if one does not exist
							if exists, err := catalog.VerifyExists(config.ConfigFile); !exists {
								if err := catalog.EnsureConfig(config); err != nil {
									return cli.Exit(config.StyledError(err), 1)
								}
								fmt.Printf("%s - created.", config.ConfigFile)
							} else if err != nil {
								return cli.Exit(config.StyledError(err), 1)
							}
							return catalog.PrintConfigSources(config)
						},
					},
				},
			},
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}

// runProgram runs a RootScreen TUI and prints its completion output after the
// program exits. The interactive form renders in the alternate screen buffer
// and is discarded on exit, so the caller prints the result below the banner
// instead of leaving selector fragments behind.
func runProgram(screen model.RootScreen) error {
	m, err := tea.NewProgram(screen).Run()
	if err != nil {
		return err
	}
	if rp, ok := m.(model.ResultProvider); ok {
		if v := rp.ResultView(); v != "" {
			fmt.Print(v)
		}
	}
	return nil
}
