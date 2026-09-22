package cmd

import (
	"fmt"

	"github.com/polymorcodeus/book/internal/book"
)

// printCatalog marshals an item in the given format and prints it.
func printCatalog[T any](item T, format string) error {
	data, err := book.MarshalCatalog(item, format)
	if err != nil {
		return err
	}
	fmt.Print(string(data))
	return nil
}
