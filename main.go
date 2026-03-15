package main

import (
	"os"

	"github.com/clankhost/clank-cli/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
