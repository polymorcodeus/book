// Package cmd implements the book CLI.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"log"
	"os"
	"path/filepath"

	"github.com/polymorcodeus/gofiglet"
	altsrc "github.com/urfave/cli-altsrc/v3"
	alttoml "github.com/urfave/cli-altsrc/v3/toml"
	validation "github.com/urfave/cli-validation"
	"github.com/urfave/cli/v3"

	"github.com/polymorcodeus/book/internal/book"
	"github.com/polymorcodeus/book/internal/catalog"
)

var (
	version = "internal"
)

// SetVersion sets the application version string used by the CLI.
func SetVersion(v string) {
	version = v
}

// Main builds and runs the book CLI application.
func Main() {
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

	var markURL string
	var markTags string
	var markTitle string

	cmd := &cli.Command{
		Name:                  "book",
		Usage:                 "mark ur life!",
		Description:           "all your marks are belong to us ...",
		Version:               version,
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
				Usage:       "set to true to enable TUI and other visual e",
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
			// Additionally, this is skipped for `catalog` as those are admin tools.
			// This is intentional: when only a subcommand is given (e.g. "book shelf"),
			// urfave/cli will auto-render the help text. We skip catalog loading so
			// help renders quickly without reading the filesystem.
			if cmd.Args().Len() > 1 {
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
						Action: func(ctx context.Context, cmd *cli.Command) error {
							if err := addShelf(&bookShelves, config); err != nil {
								return cli.Exit(config.StyledError(err), 1)
							}
							return nil
						},
					},
					{
						Name:    "list",
						Usage:   "list shelves",
						Aliases: []string{"ls"},
						Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
							if !config.Interactive && format == "" {
								return ctx, cli.Exit(config.StyledError(fmt.Errorf("set --format=[json|toml] to output shelves non-interactively")), 1)
							}
							return ctx, nil
						},
						Action: func(ctx context.Context, cmd *cli.Command) error {
							if err := shelves(&bookShelves, format, config); err != nil {
								return cli.Exit(config.StyledError(err), 1)
							}
							return nil
						},
					},
					{
						Name:    "remove",
						Usage:   "remove an existing collection from a persona",
						Aliases: []string{"rm"},
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return errors.ErrUnsupported
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
						Action: func(ctx context.Context, cmd *cli.Command) error {
							if err := addCollection(&bookShelves, config); err != nil {
								return cli.Exit(config.StyledError(err), 1)
							}
							return nil
						},
					},
					{
						Name:    "list",
						Usage:   "list collections in shelve",
						Aliases: []string{"ls"},
						Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
							if !config.Interactive && format == "" {
								return ctx, cli.Exit(config.StyledError(fmt.Errorf("set --format=[json|toml] to output collections non-interactively")), 1)
							}
							return ctx, nil
						},
						Action: func(ctx context.Context, cmd *cli.Command) error {
							if err := collections(&bookShelves, shelf, format, config); err != nil {
								return cli.Exit(config.StyledError(err), 1)
							}
							return nil
						},
					},
					{
						Name:    "remove",
						Usage:   "remove an existing collection from a persona",
						Aliases: []string{"rm"},
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return cli.Exit(config.StyledError(errors.ErrUnsupported), 1)
						},
					},
				},
			},
			{
				Name:  "mark",
				Usage: "options for bookmarks",
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
				},
				Commands: []*cli.Command{
					{
						Name:  "add",
						Usage: "add a new bookmark",
						Flags: []cli.Flag{
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
									fmt.Errorf("cannot pass URL as both --url and a positional argument")), 1)
							}

							// use positional argument to populate markURL if flag not used
							if c.Args().First() != "" {
								markURL = c.Args().First()
							}
							if markURL == "" {
								return ctx, cli.Exit(config.StyledError(
									fmt.Errorf("must pass URL as --url or argument")), 1)
							}
							return ctx, nil
						},
						Action: func(ctx context.Context, cmd *cli.Command) error {
							if err := addMark(&bookShelves, markURL, markTags, shelf, collection, markTitle, config); err != nil {
								return cli.Exit(config.StyledError(err), 1)
							}
							return nil
						},
					},
					{
						Name:  "edit",
						Usage: "edit an existing bookmark",
						Action: func(ctx context.Context, cmd *cli.Command) error {
							if err := editMark(&bookShelves, config); err != nil {
								return cli.Exit(config.StyledError(err), 1)
							}
							return nil
						},
					},
					{
						Name:  "get",
						Usage: "browse bookmarks and open selected",
						Action: func(ctx context.Context, cmd *cli.Command) error {
							if err := mark(&bookShelves, config); err != nil {
								return cli.Exit(config.StyledError(err), 1)
							}
							return nil
						},
					},
					{
						Name:    "list",
						Usage:   "list marks in a collection",
						Aliases: []string{"ls"},
						Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
							if !config.Interactive && format == "" {
								return ctx, cli.Exit(config.StyledError(fmt.Errorf("set --format=[json|toml] to output collections non-interactively")), 1)
							}
							return ctx, nil
						},
						Action: func(ctx context.Context, cmd *cli.Command) error {
							if err := marks(&bookShelves, shelf, collection, format, config); err != nil {
								return cli.Exit(config.StyledError(err), 1)
							}
							return nil
						},
					},
					{
						Name:    "remove",
						Usage:   "remove an existing bookmark",
						Aliases: []string{"rm"},
						Action: func(ctx context.Context, cmd *cli.Command) error {
							if err := removeMark(&bookShelves, config); err != nil {
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
