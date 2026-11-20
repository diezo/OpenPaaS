package main

import (
	"os"

	"github.com/diezo/openpaas/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
