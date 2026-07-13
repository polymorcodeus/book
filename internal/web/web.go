package web

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"charm.land/huh/v2/spinner"
	"github.com/PuerkitoBio/goquery"
)

func OpenURL(url string) error {
	switch runtime.GOOS {
	case "windows":
		return fmt.Errorf("yeah - this ain't gonna work on windows")
	case "darwin":
		return exec.Command("open", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}

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
		return strings.Join(strings.Fields(strings.TrimSpace(doc.Find("title").Text())), " "), nil
	case http.StatusForbidden:
		return "", nil
	case http.StatusNotFound:
		return "", fmt.Errorf("betta check yerself - that's a 4oh4!\n%s", url)
	default:
		return "", fmt.Errorf("unchecked error: %d", res.StatusCode)
	}
}

func LoadWebsite(url string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var title string
	var err error

	return title, spinner.New().
		Context(ctx).
		ActionWithErr(func(context.Context) error {
			title, err = WebsiteTitle(url)
			if err != nil {
				return err
			}
			if title != "" {
				return nil
			}
			return nil
		}).
		Title("Loading mark title ...").
		Type(spinner.Line).
		Run()
}
