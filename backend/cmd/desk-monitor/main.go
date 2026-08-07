package main

import (
	"os"

	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/bootstrap"
	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/platform/buildinfo"
)

var (
	version = "dev"
	commit  = "unknown"
	builtAt = "unknown"
)

func main() {
	info := buildinfo.New(version, commit, builtAt)
	os.Exit(bootstrap.Execute(os.Args[1:], os.Stdout, os.Stderr, info))
}
