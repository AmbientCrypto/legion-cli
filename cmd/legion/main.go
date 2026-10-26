// Command legion is the customer command line for Ambient Legion.
package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/AmbientCrypto/legion-cli/internal/cli"
)

// Set by GoReleaser: -X main.version=... -X main.commit=... -X main.date=...
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	code := cli.Execute(cli.DefaultEnv(ctx, version, commit, date), os.Args[1:])
	stop()
	os.Exit(code)
}
