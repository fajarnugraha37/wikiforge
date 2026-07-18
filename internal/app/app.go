package app

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/fajarnugraha37/wikiforge/internal/cli"
)

func Run(args []string) int {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return cli.CLI{Out: os.Stdout, Err: os.Stderr}.Run(ctx, args)
}
