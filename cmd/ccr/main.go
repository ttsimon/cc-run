package main

import (
	"os"

	"github.com/ttsimon/cc-run/internal/cli"
)

// 由 GoReleaser 通过 -ldflags 注入；本地 go build 时保持默认值。
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cli.Version = version
	cli.Commit = commit
	cli.Date = date
	os.Exit(cli.Execute(os.Args[1:]))
}
