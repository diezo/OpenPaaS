package main

import (
	"os"

	"github.com/openpaas/openpaas/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
