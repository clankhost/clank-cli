package main

import (
	"os"

	"github.com/anaremore/clank/apps/cli/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
