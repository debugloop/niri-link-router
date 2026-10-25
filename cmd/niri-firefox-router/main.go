package main

import (
	"errors"
	"flag"
	"fmt"
	"niri-firefox-router/internal/cli"
	"os"
)

func main() {
	if err := (cli.Runner{Out: os.Stdout, Err: os.Stderr}).Run(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, "niri-firefox-router:", err)
		os.Exit(1)
	}
}
