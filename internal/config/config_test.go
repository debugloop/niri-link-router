package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigPrecedence(t *testing.T) {
	dir := t.TempDir()
	system := filepath.Join(dir, "system.json")
	user := filepath.Join(dir, "user.json")
	check := func(want string) {
		t.Helper()
		got, err := load(system, user)
		if err != nil || got.FallbackWorkspace != want {
			t.Fatalf("got %+v, %v; want %q", got, err, want)
		}
	}
	write := func(path, contents string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	check("web")
	write(system, `{"fallback_workspace":"browser"}`)
	check("browser")
	write(user, `{"fallback_workspace":"internet"}`)
	check("internet")
}

func TestInvalidConfig(t *testing.T) {
	for _, data := range []string{
		``, `null`, `[]`, `{}`, `{"fallback_workspace":null}`,
		`{"fallback_workspace":12}`, `{"fallback_workspace":""}`,
		`{"fallback_workspace":"  "}`, `{"fallback_workspace":"a\nb"}`,
		`{"fallback_workspace":"web","typo":true}`,
		`{"fallback_workspace":"web"} {}`, `{"fallback_workspace":"web"} garbage`,
	} {
		t.Run(data, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			if err := os.WriteFile(path, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := load(path); err == nil {
				t.Fatal("accepted invalid config")
			}
		})
	}
}

func TestConfigReadFailure(t *testing.T) {
	if _, err := load(t.TempDir()); err == nil {
		t.Fatal("ignored unreadable config")
	}
}

func TestWorkspaceNames(t *testing.T) {
	for _, name := range []string{"web", "internet browser", "浏览器"} {
		if err := (Settings{FallbackWorkspace: name}).Validate(); err != nil {
			t.Fatal(err)
		}
	}
}
