package main

import (
	"os"

	"github.com/fluxplane/fluxplane-dex/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args[1:]))
}
