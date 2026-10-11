// Command dotclaude shows and checks a Claude Code configuration.
package main

import (
	"fmt"
	"os"

	"github.com/mdbtq/dotclaude/internal/cli"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := cli.Execute(version); err != nil {
		fmt.Fprintln(os.Stderr, "dotclaude:", err)
		os.Exit(1)
	}
}
