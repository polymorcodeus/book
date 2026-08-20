// Package web is used to grep web title if not provided and handles basic
// open method
package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"charm.land/huh/v2/spinner"
	"github.com/PuerkitoBio/goquery"
)

// ErrTitleUnavailable is returned when a page title cannot be fetched
// automatically (e.g. HTTP 403 or an empty <title> tag). Callers should
// prompt the user to enter a title manually.
var ErrTitleUnavailable = errors.New("couldn't fetch title")

// OpenURL opens the given URL in the default browser.
func OpenURL(url string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("cmd", "/c", "start", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}

// WebsiteTitle fetches and extracts the page title from a URL.
func WebsiteTitle(url string) (string, error) {
	res, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer func() { _ = res.Body.Close() }()

	switch res.StatusCode {
	case http.StatusOK:
		doc, err := goquery.NewDocumentFromReader(res.Body)
		if err != nil {
			return "", err
		}
		title := strings.Join(strings.Fields(strings.TrimSpace(doc.Find("title").Text())), " ")
		if title == "" {
			return "", ErrTitleUnavailable
		}
		return title, nil
	case http.StatusForbidden:
		return "", ErrTitleUnavailable
	case http.StatusNotFound:
		return "", fmt.Errorf("betta check yerself - that's a 4oh4!\n%s", url)
	default:
		return "", fmt.Errorf("unchecked error: %d", res.StatusCode)
	}
}

// LoadWebsite fetches a page title with a spinner and 10-second timeout.
func LoadWebsite(url string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var title string
	var err error

	return title, spinner.New().
		Context(ctx).
		ActionWithErr(func(context.Context) error {
			title, err = WebsiteTitle(url)
			return err
		}).
		Title("Loading mark title ...").
		Type(spinner.Line).
		Run()
}
