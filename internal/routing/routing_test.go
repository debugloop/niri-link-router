package routing

import "testing"

func ptr[T any](v T) *T { return &v }
func ws(id uint64, idx int, out string, focused bool) Workspace {
	return Workspace{ID: id, Index: idx, Output: ptr(out), Focused: focused}
}
func named(w Workspace, name string) Workspace { w.Name = ptr(name); return w }
func win(id, wid uint64, fid string) Window {
	return Window{ID: id, PID: 100, WorkspaceID: ptr(wid), AppID: "firefox", Title: "[nfr:" + fid + "] Page"}
}
func TestRouting(t *testing.T) {
	tests := []struct {
		name         string
		spaces       []Workspace
		windows      []Window
		want         uint64
		normal, fail bool
	}{
		{"current workspace", []Workspace{ws(0, 5, "A", true), named(ws(90, 4, "A", false), "web")}, []Window{win(1, 0, "1"), win(2, 90, "2")}, 1, false, false},
		{"fallback not nearest", []Workspace{ws(900, 5, "A", true), ws(1, 4, "A", false), named(ws(899, 1, "A", false), "web")}, []Window{win(1, 1, "1"), win(2, 899, "2")}, 2, false, false},
		{"reorder keeps fallback", []Workspace{ws(900, 5, "A", true), ws(1, 1, "A", false), named(ws(899, 4, "A", false), "web")}, []Window{win(1, 1, "1"), win(2, 899, "2")}, 2, false, false},
		{"web on another output", []Workspace{ws(0, 5, "A", true), ws(1, 4, "A", false), named(ws(2, 7, "B", false), "web")}, []Window{win(1, 1, "1"), win(2, 2, "2")}, 2, false, false},
		{"current wins across outputs", []Workspace{ws(0, 5, "A", true), named(ws(2, 5, "B", false), "web")}, []Window{win(1, 0, "1"), win(2, 2, "2")}, 1, false, false},
		{"focused window tie", []Workspace{ws(0, 5, "A", true)}, []Window{win(1, 0, "1"), func() Window { w := win(2, 0, "2"); w.Focused = true; return w }()}, 2, false, false},
		{"recent window tie", []Workspace{ws(0, 5, "A", true)}, []Window{win(1, 0, "1"), func() Window { w := win(2, 0, "2"); w.FocusTime = &Timestamp{Seconds: 1}; return w }()}, 2, false, false},
		{"deterministic tie", []Workspace{ws(0, 5, "A", true)}, []Window{win(9, 0, "1"), win(8, 0, "2")}, 8, false, false},
		{"no browser", []Workspace{ws(0, 1, "A", true)}, nil, 0, true, false},
		{"no current or web browser", []Workspace{ws(0, 1, "A", true), ws(1, 2, "A", false)}, []Window{win(1, 1, "1")}, 0, true, false},
		{"unlabelled current not skipped", []Workspace{ws(0, 1, "A", true), named(ws(1, 2, "A", false), "web")}, []Window{func() Window { w := win(1, 0, "1"); w.Title = "Page"; return w }(), win(2, 1, "2")}, 1, false, false},
		{"forged title other app", []Workspace{ws(0, 1, "A", true)}, []Window{{AppID: "kitty", Title: "[nfr:1] Page"}}, 0, true, false},
		{"duplicate IDs different profiles", []Workspace{ws(0, 1, "A", true), named(ws(1, 2, "A", false), "web")}, []Window{win(1, 0, "1"), func() Window { w := win(2, 1, "1"); w.PID = 200; return w }()}, 1, false, false},
		{"duplicate IDs same profile", []Workspace{ws(0, 1, "A", true)}, []Window{win(1, 0, "1"), win(2, 0, "1")}, 0, false, true},
		{"unknown workspace", []Workspace{ws(0, 1, "A", true)}, []Window{win(1, 2, "1")}, 0, false, true},
		{"no focus", []Workspace{ws(0, 1, "A", false)}, nil, 0, false, true},
		{"duplicate web names", []Workspace{ws(0, 1, "A", true), named(ws(1, 2, "A", false), "web"), named(ws(2, 3, "B", false), "web")}, nil, 0, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := Select(tt.spaces, tt.windows, "web")
			if (err != nil) != tt.fail {
				t.Fatalf("error %v", err)
			}
			if err == nil && (d.NiriID != tt.want || d.Normal != tt.normal) {
				t.Fatalf("got %+v", d)
			}
		})
	}
}
func TestFirefoxIDs(t *testing.T) {
	for _, title := range []string{"[nfr:0] X", "[nfr:2147483648] X", "[nfr:01] X", "[nfr:-1] X", "[nfr:1] [nfr:2] X", "Page [nfr:1] ", "[nfr:2]X"} {
		if _, err := FirefoxID(title); err == nil {
			t.Errorf("accepted %q", title)
		}
	}
}
func TestRecentTimestamp(t *testing.T) {
	if recent(&Timestamp{1, 2}, &Timestamp{1, 1}) <= 0 || recent(nil, &Timestamp{}) >= 0 || recent(nil, nil) != 0 {
		t.Fatal("timestamp ordering")
	}
}

func TestConfiguredFallback(t *testing.T) {
	spaces := []Workspace{ws(0, 1, "A", true), named(ws(1, 2, "A", false), "web"), named(ws(2, 3, "B", false), "browser")}
	windows := []Window{win(10, 1, "1"), win(20, 2, "2")}
	d, err := Select(spaces, windows, "browser")
	if err != nil || d.NiriID != 20 {
		t.Fatalf("got %+v, %v", d, err)
	}
	windows = append(windows, win(30, 0, "3"))
	d, err = Select(spaces, windows, "browser")
	if err != nil || d.NiriID != 30 {
		t.Fatalf("current workspace lost priority: %+v, %v", d, err)
	}
}

func TestConfiguredFallbackErrors(t *testing.T) {
	spaces := []Workspace{ws(0, 1, "A", true), named(ws(1, 2, "A", false), "web")}
	if d, err := Select(spaces, []Window{win(10, 1, "1")}, "browser"); err != nil || !d.Normal {
		t.Fatalf("expected normal Firefox instead of hardcoded web: %+v, %v", d, err)
	}
	spaces = append(spaces, named(ws(2, 3, "A", false), "browser"), named(ws(3, 4, "B", false), "browser"))
	if _, err := Select(spaces, nil, "browser"); err == nil {
		t.Fatal("accepted duplicate fallback names")
	}
}
