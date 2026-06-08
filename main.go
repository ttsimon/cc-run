package main

import (
	"os"

	"ccr/internal/cli"
)

func main() {
	os.Exit(cli.Execute(os.Args[1:]))
}
