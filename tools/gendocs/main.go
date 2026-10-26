// Command gendocs writes the markdown reference for every legion command:
// go run ./tools/gendocs [dir]  (default docs/reference).
package main

import (
	"log"
	"os"

	"github.com/spf13/cobra/doc"

	"github.com/AmbientCrypto/legion-cli/internal/cli"
)

func main() {
	dir := "docs/reference"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	root := cli.Root()
	root.DisableAutoGenTag = true
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Fatal(err)
	}
	if err := doc.GenMarkdownTree(root, dir); err != nil {
		log.Fatal(err)
	}
}
