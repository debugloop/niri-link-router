package main

import (
	"context"
	"fmt"
	"niri-firefox-router/internal/bridge"
	"niri-firefox-router/internal/profile"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	pid, path, err := profile.Default().BrowserAncestor(os.Getppid())
	if err == nil {
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		err = bridge.RunHost(ctx, bridge.Identity{FirefoxPID: pid, ProfilePath: path}, os.Stdin, os.Stdout)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "niri-firefox-router native host:", err)
		os.Exit(1)
	}
}
