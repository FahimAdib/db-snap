package main

import (
	"os"

	"db-snap/internal/cli"
)

var version = "0.1.0"

func main() {
	root := cli.NewRoot(version)
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
