package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"niri-firefox-router/internal/bridge"
	"niri-firefox-router/internal/config"
	"niri-firefox-router/internal/niri"
	"niri-firefox-router/internal/profile"
	"niri-firefox-router/internal/protocol"
	"niri-firefox-router/internal/routing"
	"os"
	"os/exec"
	"strconv"
	"time"
)

type Runner struct {
	Out            io.Writer
	Err            io.Writer
	Start          func(string, []string) error
	ResolveProfile func(int) (string, error)
}

func Start(executable string, args []string) error {
	command := exec.Command(executable, args...)
	if err := command.Start(); err != nil {
		return errors.New("cannot start Firefox executable")
	}
	go func() { _ = command.Wait() }()
	return nil
}
func (r Runner) Run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: niri-firefox-router open [options] URL | focus [options]")
	}
	command := args[0]
	if command == "help" || command == "--help" || command == "-h" {
		_, err := fmt.Fprintln(r.Out, "Usage: niri-firefox-router COMMAND [OPTIONS]\n\nCommands:\n  open URL    Open in the selected Firefox window, otherwise use Firefox normally\n  focus       Focus Firefox on this workspace, otherwise the configured fallback\n\nPut options before the URL. Use COMMAND --help for its options.")
		return err
	}
	if command != "open" && command != "focus" {
		return errors.New("unknown command")
	}
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(r.Err)
	flags.String("fallback-workspace", config.DefaultFallbackWorkspace, "fallback workspace name (overrides config)")
	dry := flags.Bool("dry-run", false, "print window selection only, without a URL or delivery")
	firefox := flags.String("firefox", "firefox", "Firefox executable for ordinary fallback (one path)")
	var sourceID *uint64
	var background bool
	if command == "open" {
		flags.BoolVar(&background, "bg", false, "mark the destination urgent without focusing it")
		flags.Func("source-window-id", "Niri ID of the window that opened the link", func(value string) error {
			id, err := strconv.ParseUint(value, 10, 64)
			if err == nil {
				sourceID = &id
			}
			return err
		})
	}
	inactive := flags.Bool("inactive", false, "do not select the routed destination tab")
	timeout := flags.Duration("timeout", 5*time.Second, "niri and native bridge request timeout")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if *timeout <= 0 || *timeout > protocol.MaxLifetime {
		return errors.New("timeout must be positive and at most 30s")
	}
	raw := ""
	if command == "open" {
		if flags.NArg() != 1 {
			return errors.New("open needs one URL. Put all options before the URL")
		}
		raw = flags.Arg(0)
		if err := protocol.ValidateURL(raw); err != nil {
			return err
		}
	} else if flags.NArg() != 0 {
		return errors.New("focus takes no positional arguments")
	}
	ordinary := func(cause error) error {
		if command != "open" {
			return cause
		}
		if r.Err != nil {
			_, _ = fmt.Fprintln(r.Err, "Routing unavailable. Using Firefox normally.")
		}
		if *dry {
			return json.NewEncoder(r.Out).Encode(routing.Decision{Normal: true})
		}
		start := r.Start
		if start == nil {
			start = Start
		}
		if err := start(*firefox, []string{raw}); err != nil {
			return err
		}
		_, err := fmt.Fprintln(r.Out, "Dispatched to Firefox normally. Delivery is not acknowledged.")
		return err
	}
	fallback, err := fallbackWorkspace(flags)
	if err != nil {
		return ordinary(err)
	}
	compositor := niri.Client{Socket: os.Getenv("NIRI_SOCKET"), Timeout: *timeout}
	spaces, windows, err := compositor.Snapshot()
	if err != nil {
		return ordinary(err)
	}
	var decision routing.Decision
	if command == "focus" {
		decision, err = routing.Select(spaces, windows, fallback)
	} else {
		if sourceID == nil {
			sourceID = sourceWindow(windows)
		}
		decision, err = routing.SelectForOpen(spaces, windows, fallback, sourceID)
	}
	if err != nil {
		return ordinary(err)
	}
	if decision.Normal {
		return ordinary(errors.New("no eligible Firefox window"))
	}
	if command == "focus" {
		if *dry {
			return json.NewEncoder(r.Out).Encode(decision)
		}
		return compositor.Focus(decision.NiriID)
	}
	if decision.FirefoxID == 0 {
		return ordinary(errors.New("destination has no router window ID"))
	}
	if *dry {
		return json.NewEncoder(r.Out).Encode(decision)
	}
	resolve := r.ResolveProfile
	if resolve == nil {
		resolve = profile.Default().Resolve
	}
	path, err := resolve(decision.FirefoxPID)
	if err != nil {
		return ordinary(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	native, err := bridge.Dial(ctx, path, decision.FirefoxPID)
	if err != nil {
		return ordinary(err)
	}
	defer func() { _ = native.Close() }()
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return ordinary(errors.New("cannot generate native request ID"))
	}
	deadline, _ := ctx.Deadline()
	request := protocol.Request{Version: protocol.Version, Type: "open", ID: hex.EncodeToString(nonce[:]), WindowID: decision.FirefoxID, URL: raw, Active: !*inactive, Expires: deadline.UnixMilli()}
	if err := native.Open(request); err != nil {
		if bridge.MayHaveDelivered(err) {
			return err
		}
		return ordinary(err)
	}
	notify := compositor.Focus
	if background {
		notify = compositor.SetUrgent
	}
	if err := notify(decision.NiriID); err != nil && r.Err != nil {
		_, _ = fmt.Fprintf(r.Err, "Tab opened, but Niri could not update the destination window: %v\n", err)
	}
	_, err = fmt.Fprintln(r.Out, "Opened in Firefox. The extension acknowledged tab creation.")
	return err
}
