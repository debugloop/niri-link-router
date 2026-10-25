package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

const DefaultFallbackWorkspace = "web"

type Settings struct {
	FallbackWorkspace string `json:"fallback_workspace"`
}

func (s Settings) Validate() error {
	if strings.TrimSpace(s.FallbackWorkspace) == "" || strings.ContainsFunc(s.FallbackWorkspace, unicode.IsControl) {
		return errors.New("fallback workspace must be a nonempty name without control characters")
	}
	return nil
}

func Load() (Settings, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return Settings{}, errors.New("cannot find the user config directory")
	}
	return load("/etc/niri-firefox-router/config.json", filepath.Join(dir, "niri-firefox-router", "config.json"))
}

func load(paths ...string) (Settings, error) {
	settings := Settings{FallbackWorkspace: DefaultFallbackWorkspace}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return Settings{}, fmt.Errorf("cannot read router config %s: %w", path, err)
		}
		var value struct {
			FallbackWorkspace *string `json:"fallback_workspace"`
		}
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&value); err != nil {
			return Settings{}, fmt.Errorf("invalid router config %s: %w", path, err)
		}
		if value.FallbackWorkspace == nil {
			return Settings{}, fmt.Errorf("router config %s needs fallback_workspace", path)
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			return Settings{}, fmt.Errorf("router config %s must contain one JSON object", path)
		}
		settings.FallbackWorkspace = *value.FallbackWorkspace
		if err := settings.Validate(); err != nil {
			return Settings{}, fmt.Errorf("invalid router config %s: %w", path, err)
		}
	}
	return settings, nil
}
