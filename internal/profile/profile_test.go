package profile

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func put(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}
func fixture(t *testing.T, args []string) (Resolver, string) {
	t.Helper()
	root := t.TempDir()
	r := Resolver{Proc: filepath.Join(root, "proc"), Roots: []string{filepath.Join(root, "profiles")}}
	cmd := "/usr/bin/firefox\x00"
	for _, a := range args {
		cmd += a + "\x00"
	}
	put(t, filepath.Join(r.Proc, "123/cmdline"), cmd)
	profile := filepath.Join(r.Roots[0], "personal")
	if err := os.MkdirAll(profile, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("127.0.0.2:+123", filepath.Join(profile, "lock")); err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(r.Roots[0], "profiles.ini"), "[General]\nVersion=2\n[Profile0]\nName=private\nIsRelative=1\nPath=personal\n[Profile1]\nName=default\nIsRelative=1\nPath=work\nDefault=1\n")
	if err := os.MkdirAll(filepath.Join(r.Roots[0], "work"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("127.0.0.2:+456", filepath.Join(r.Roots[0], "work/lock")); err != nil {
		t.Fatal(err)
	}
	return r, profile
}
func TestRegisteredProfileByPID(t *testing.T) {
	for _, args := range [][]string{nil, {"-P", "private"}, {"--name", "firefox"}} {
		r, want := fixture(t, args)
		got, err := r.Resolve(123)
		if err != nil || got != want {
			t.Fatalf("got %q %v", got, err)
		}
	}
}
func TestExplicitUnregisteredProfile(t *testing.T) {
	for _, form := range []string{"--profile", "--profile=", "-profile"} {
		t.Run(form, func(t *testing.T) {
			r, want := fixture(t, nil)
			_ = os.Remove(filepath.Join(r.Roots[0], "profiles.ini"))
			args := "/usr/bin/firefox\x00" + form
			if form == "--profile=" {
				args += want
			} else {
				args += "\x00" + want
			}
			put(t, filepath.Join(r.Proc, "123/cmdline"), args+"\x00")
			got, err := r.Resolve(123)
			if err != nil || got != want {
				t.Fatal(got, err)
			}
		})
	}
}
func TestRelativeExplicitProfile(t *testing.T) {
	r, want := fixture(t, []string{"--profile", "personal"})
	_ = os.Remove(filepath.Join(r.Roots[0], "profiles.ini"))
	_ = os.Symlink(r.Roots[0], filepath.Join(r.Proc, "123/cwd"))
	got, err := r.Resolve(123)
	if err != nil || got != want {
		t.Fatal(got, err)
	}
}
func TestRejectMissingOrAmbiguousLocks(t *testing.T) {
	for _, which := range []string{"stale", "missing", "ambiguous"} {
		t.Run(which, func(t *testing.T) {
			r, p := fixture(t, nil)
			_ = os.Remove(filepath.Join(p, "lock"))
			if which == "stale" {
				_ = os.Symlink("127.0.0.2:+999", filepath.Join(p, "lock"))
			}
			if which == "ambiguous" {
				_ = os.Symlink("127.0.0.2:+123", filepath.Join(p, "lock"))
				other := filepath.Join(r.Roots[0], "work/lock")
				_ = os.Remove(other)
				_ = os.Symlink("127.0.0.2:+123", other)
			}
			if _, err := r.Resolve(123); err == nil {
				t.Fatal("accepted unsafe profile resolution")
			}
		})
	}
}
func TestNativeProfilesAndOtherProcess(t *testing.T) {
	for _, args := range [][]string{{"--no-remote"}, {"-no-remote"}} {
		r, _ := fixture(t, args)
		if _, err := r.Resolve(123); err != nil {
			t.Fatal("native transport must support no-remote profiles", err)
		}
	}
	r, _ := fixture(t, nil)
	put(t, filepath.Join(r.Proc, "123/cmdline"), "/usr/bin/kitty\x00")
	if _, err := r.Resolve(123); err == nil {
		t.Fatal("accepted another executable")
	}
	for _, pid := range []int{0, -1, 999} {
		if _, err := r.Resolve(pid); err == nil {
			t.Fatal(strconv.Itoa(pid))
		}
	}
}
func TestRegistryBound(t *testing.T) {
	r, _ := fixture(t, nil)
	put(t, filepath.Join(r.Roots[0], "profiles.ini"), string(make([]byte, bound+1)))
	if _, err := r.Resolve(123); err == nil {
		t.Fatal("accepted oversized metadata")
	}
}

func TestBrowserAncestor(t *testing.T) {
	r, want := fixture(t, nil)
	put(t, filepath.Join(r.Proc, "789/cmdline"), "/usr/bin/native-host\x00")
	put(t, filepath.Join(r.Proc, "789/status"), "PPid:\t456\n")
	put(t, filepath.Join(r.Proc, "456/cmdline"), "/usr/bin/forkserver\x00")
	put(t, filepath.Join(r.Proc, "456/status"), "PPid:\t123\n")
	pid, path, err := r.BrowserAncestor(789)
	if err != nil || pid != 123 || path != want {
		t.Fatalf("got %d %q %v", pid, path, err)
	}
	put(t, filepath.Join(r.Proc, "456/status"), "PPid:\t789\n")
	if _, _, err := r.BrowserAncestor(789); err == nil {
		t.Fatal("accepted cyclic parent chain")
	}
}
