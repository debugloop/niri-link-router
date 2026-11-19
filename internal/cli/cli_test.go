package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func compositor(t *testing.T, browser bool) *[]string {
	t.Helper()
	spaces := []any{map[string]any{"id": 0, "idx": 1, "output": "A", "is_focused": true}}
	wins := []any{}
	if browser {
		wins = append(wins, map[string]any{"id": 900, "pid": 123, "workspace_id": 0, "title": "[nfr:3] Page", "app_id": "firefox"})
	}
	return compositorSnapshot(t, spaces, wins)
}
func compositorSnapshot(t *testing.T, spaces, wins []any) *[]string {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	p := filepath.Join(t.TempDir(), "socket")
	l, err := net.Listen("unix", p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	t.Setenv("NIRI_SOCKET", p)
	requests := new([]string)
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			line, _ := bufio.NewReader(c).ReadString('\n')
			*requests = append(*requests, line)
			response := any("Handled")
			if strings.Contains(line, "Workspaces") {
				response = map[string]any{"Workspaces": spaces}
			}
			if strings.Contains(line, "Windows") {
				response = map[string]any{"Windows": wins}
			}
			_ = json.NewEncoder(c).Encode(map[string]any{"Ok": response})
			_ = c.Close()
		}
	}()
	return requests
}

func TestDryRun(t *testing.T) {
	compositor(t, true)
	var out bytes.Buffer
	r := Runner{Out: &out, Err: &out, Start: func(string, []string) error { t.Fatal("dry-run launched Firefox"); return nil }}
	if err := r.Run([]string{"open", "--dry-run", "--source-window-id", "900", "https://example.org/private"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "private") || !strings.Contains(out.String(), `"firefox_window_id":3`) {
		t.Fatal(out.String())
	}
}
func TestFocusUsesCompositor(t *testing.T) {
	requests := compositor(t, true)
	var out bytes.Buffer
	if err := (Runner{Out: &out, Err: &out}).Run([]string{"focus"}); err != nil {
		t.Fatal(err)
	}
	if len(*requests) != 4 || !strings.Contains((*requests)[3], `"id":900`) {
		t.Fatal(*requests)
	}
}
func TestUnroutableLinksUseOrdinaryFirefox(t *testing.T) {
	for _, browser := range []bool{false, true} {
		compositor(t, browser)
		t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
		var out bytes.Buffer
		calls := 0
		raw := "https://example.org/a+b?q=%25#original"
		r := Runner{Out: &out, Err: &out, ResolveProfile: func(int) (string, error) { return "/profiles/personal", nil }, Start: func(exe string, args []string) error {
			calls++
			if exe != "custom-firefox" || len(args) != 1 || args[0] != raw {
				t.Fatal("not a normal Firefox invocation", exe, args)
			}
			return nil
		}}
		if err := r.Run([]string{"open", "--firefox", "custom-firefox", raw}); err != nil {
			t.Fatal(err)
		}
		if calls != 1 {
			t.Fatal("wrong number of launches", calls)
		}
	}
}
func TestFallbackFromOtherWorkspace(t *testing.T) {
	compositorSnapshot(t, []any{map[string]any{"id": 0, "idx": 1, "output": "A", "is_focused": true}, map[string]any{"id": 1, "idx": 2, "output": "A", "name": "unrelated"}}, []any{map[string]any{"id": 10, "pid": 123, "workspace_id": 1, "app_id": "firefox"}})
	var out bytes.Buffer
	calls := 0
	r := Runner{Out: &out, Err: &out, Start: func(string, []string) error { calls++; return nil }}
	if err := r.Run([]string{"open", "https://example.org/"}); err != nil || calls != 1 {
		t.Fatal(err, calls)
	}
}
func TestNoNiriDryRun(t *testing.T) {
	t.Setenv("NIRI_SOCKET", filepath.Join(t.TempDir(), "absent"))
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var out, diagnostics bytes.Buffer
	if err := (Runner{Out: &out, Err: &diagnostics}).Run([]string{"open", "--dry-run", "https://example.org/private"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"normal_launch":true`) || strings.Contains(out.String()+diagnostics.String(), "private") {
		t.Fatal("bad diagnostic output")
	}
}
func TestConfiguredFallback(t *testing.T) {
	compositorSnapshot(t, []any{map[string]any{"id": 0, "idx": 1, "output": "A", "is_focused": true}, map[string]any{"id": 1, "idx": 2, "output": "B", "name": "browser"}}, []any{map[string]any{"id": 10, "pid": 123, "workspace_id": 1, "app_id": "firefox", "title": "[nfr:1] Page"}})
	dir := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "niri-firefox-router")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"fallback_workspace":"browser"}`), 0600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	r := Runner{Out: &out, Err: &out}
	if err := r.Run([]string{"open", "--dry-run", "https://example.org/"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"niri_window_id":10`) {
		t.Fatal("ignored configured workspace")
	}
	out.Reset()
	if err := r.Run([]string{"open", "--dry-run", "--fallback-workspace", "elsewhere", "https://example.org/"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"normal_launch":true`) {
		t.Fatal("ignored CLI override")
	}
}
func TestArgumentErrorsAndRemovedCommands(t *testing.T) {
	for _, args := range [][]string{nil, {"pair"}, {"keygen"}, {"open"}, {"focus", "extra"}, {"open", "--transport", "bare", "https://example.org/"}, {"open", "--key-file", "key", "https://example.org/"}, {"open", "--timeout", "0s", "https://example.org/"}, {"open", "file:///etc/passwd"}} {
		var out bytes.Buffer
		r := Runner{Out: &out, Err: &out, Start: func(string, []string) error { t.Fatal("invalid arguments launched Firefox"); return nil }}
		if err := r.Run(args); err == nil {
			t.Fatal("accepted invalid or abolished interface", args)
		}
	}
}
func TestOrdinaryLaunchFailureIsNotRetried(t *testing.T) {
	compositor(t, false)
	var out bytes.Buffer
	calls := 0
	failure := errors.New("launch failed")
	r := Runner{Out: &out, Err: &out, Start: func(string, []string) error { calls++; return failure }}
	if err := r.Run([]string{"open", "https://example.org/"}); !errors.Is(err, failure) || calls != 1 {
		t.Fatal("retried or masked error", err, calls)
	}
}
