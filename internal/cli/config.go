package cli

import (
	"flag"
	"niri-firefox-router/internal/config"
)

func fallbackWorkspace(flags *flag.FlagSet) (string, error) {
	settings, err := config.Load()
	if err != nil {
		return "", err
	}
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "fallback-workspace" {
			settings.FallbackWorkspace = f.Value.String()
		}
	})
	return settings.FallbackWorkspace, settings.Validate()
}
