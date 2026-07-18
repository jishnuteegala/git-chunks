// git-chunks commits and optionally pushes pending changes in small chunks.
package main

import (
	"os"

	"github.com/jishnuteegala/git-chunks/internal/cli"
)

var version = "dev"

func main() {
	os.Exit(cli.Main(os.Args[1:], os.Stdout, os.Stderr, version))
}
