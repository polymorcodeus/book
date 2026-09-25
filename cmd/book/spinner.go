package cmd

import (
	"context"
	"time"

	"charm.land/huh/v2/spinner"

	"github.com/polymorcodeus/book/pkg/book"
	"github.com/polymorcodeus/book/pkg/catalog"
	"github.com/polymorcodeus/book/pkg/web"
)

// loadCatalog loads shelves with an optional spinner when running interactively.
func loadCatalog(bs *book.BookShelves, config *book.Config, interactive bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if !interactive {
		return catalog.LoadShelves(bs, catalog.PathsFromConfig(config))
	}

	return spinner.New().
		Context(ctx).
		ActionWithErr(func(context.Context) error {
			time.Sleep(1 * time.Second)
			return catalog.LoadShelves(bs, catalog.PathsFromConfig(config))
		}).
		Title("Loading your bookshelves ...").
		Run()
}

// loadWebsite fetches a page title with a spinner and 10-second timeout. The
// provided context is honoured and capped at 10 seconds.
func loadWebsite(ctx context.Context, url string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var title string
	var err error

	return title, spinner.New().
		Context(ctx).
		ActionWithErr(func(context.Context) error {
			title, err = web.WebsiteTitle(ctx, url)
			return err
		}).
		Title("Loading mark title ...").
		Type(spinner.Line).
		Run()
}
