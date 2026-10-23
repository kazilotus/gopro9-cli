// Command gopro9 controls a GoPro HERO9 Black over a USB-C cable.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/kazilotus/gopro9-cli/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := cli.Main(ctx, os.Args[1:], cli.Stdio{In: os.Stdin, Out: os.Stdout, Err: os.Stderr})
	stop()
	os.Exit(code)
}
