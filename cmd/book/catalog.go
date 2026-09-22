package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/BurntSushi/toml"

	"github.com/polymorcodeus/book/internal/book"
	"github.com/polymorcodeus/book/internal/catalog"
	"github.com/polymorcodeus/book/internal/theme"
)

// dumpDefaults serialises the built-in theme or template defaults as indented JSON.
func dumpDefaults(config *book.Config, dump string) (err error) {
	var jsonData []byte
	var (
		tmpPath,
		writePath string
	)

	switch dump {
	case "theme":
		writePath, err = catalog.ResolveWritePath(config.ThemeFile) // = not :=
		if err != nil {
			return err
		}
		tmpPath = writePath + ".tmp"

		if jsonData, err = json.MarshalIndent(theme.DefaultThemeConfig(), "", "  "); err != nil {
			return err
		}
	case "template":
		writePath, err = catalog.ResolveWritePath(config.TemplateFile) // = not :=
		if err != nil {
			return err
		}
		tmpPath = writePath + ".tmp"

		if jsonData, err = json.MarshalIndent(book.DefaultViewTemplates, "", "  "); err != nil {
			return err
		}
	}

	if !config.Autoconfirm {
		return fmt.Errorf("set --confirm to create %s file %s", dump, tmpPath)
	}

	if exists, _ := catalog.VerifyExists(writePath); exists {
		return os.ErrExist
	}

	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
		if err != nil {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err = f.Write(jsonData); err != nil {
		return err
	}

	return os.Rename(tmpPath, writePath)
}

// printConfigSources prints rendered configuration when config file is present
func printConfigSources(config *theme.UIConfig) error {
	var fileCfg book.FileConfig
	if _, err := toml.DecodeFile(config.ConfigFile, &fileCfg); err != nil {
		return err
	}

	heading := config.Theme.Style("highlight").Render("book config file exists - rendered configuration")
	fmt.Println(heading)
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	_, _ = fmt.Fprintln(w, "FIELD\tFILE VALUE\tEFFECTIVE")
	_, _ = fmt.Fprintln(w, "-----\t----------\t---------")

	// bool helper
	printBool := func(name string, fileVal *bool, effVal bool) {
		var f string
		if fileVal == nil {
			f = "<unset>"
		} else {
			f = fmt.Sprintf("%t", *fileVal)
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%t\n", name, f, effVal)
	}

	printBool("autoconfirm", fileCfg.Autoconfirm, config.Autoconfirm)
	printBool("interactive", fileCfg.Interactive, config.Interactive)

	// string helper
	printString := func(name, fileVal, effVal string) {
		if fileVal == "" {
			fileVal = "<unset>"
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", name, fileVal, effVal)
	}

	printString("catalog_format", fileCfg.CatalogFormat, config.CatalogFormat)
	printString("shelf_directory", fileCfg.ShelfRoot, config.ShelfRoot)
	printString("theme_file", fileCfg.ThemeFile, config.ThemeFile)
	printString("template_file", fileCfg.TemplateFile, config.TemplateFile)

	_ = w.Flush()
	return nil
}
