package bridge

import (
	"context"
	"niri-firefox-router/internal/protocol"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func runtimeDirectory(t *testing.T) {
	t.Helper()
	dir, err := os.MkdirTemp("", "nfr-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	t.Setenv("XDG_RUNTIME_DIR", dir)
}
func nativeHost(t *testing.T, profile string, pid int, status string) (func(), <-chan protocol.Request) {
	t.Helper()
	input, browserWrite := pipePair(t)
	browserRead, output := pipePair(t)
	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan error, 1)
	go func() { finished <- RunHost(ctx, Identity{FirefoxPID: pid, ProfilePath: profile}, input, output) }()
	if err := protocol.WriteFrame(browserWrite, protocol.Hello{Version: 1, Type: "hello"}); err != nil {
		t.Fatal(err)
	}
	var ready protocol.Hello
	if err := protocol.ReadFrame(browserRead, &ready); err != nil || ready.Type != "ready" {
		t.Fatal(ready, err)
	}
	requests := make(chan protocol.Request, 32)
	go func() {
		for {
			var request protocol.Request
			if err := protocol.ReadFrame(browserRead, &request); err != nil {
				return
			}
			requests <- request
			if status == "disconnect" {
				_ = browserWrite.Close()
				return
			}
			reply := protocol.Reply{Version: 1, Type: "result", ID: request.ID, Status: status}
			if status == "wrong-id" {
				reply.ID = strings.Repeat("f", 32)
				reply.Status = "opened"
			}
			if err := protocol.WriteFrame(browserWrite, reply); err != nil {
				return
			}
		}
	}()
	var once sync.Once
	stop := func() {
		once.Do(func() {
			cancel()
			_ = browserWrite.Close()
			_ = browserRead.Close()
			_ = output.Close()
			select {
			case err := <-finished:
				if err != nil && status != "wrong-id" {
					t.Error(err)
				}
			case <-time.After(2 * time.Second):
				t.Error("native host did not exit after Firefox disconnected")
			}
		})
	}
	t.Cleanup(stop)
	return stop, requests
}
func packet(id string) protocol.Request {
	return protocol.Request{Version: 1, Type: "open", ID: strings.Repeat(id, 32), WindowID: 1, URL: "https://example.org/native?q=%25#original", Active: true, Expires: time.Now().Add(10 * time.Second).UnixMilli()}
}
func TestNativeHostDeliveryAndProfileIsolation(t *testing.T) {
	runtimeDirectory(t)
	first, second := filepath.Join(t.TempDir(), "one"), filepath.Join(t.TempDir(), "two")
	_, one := nativeHost(t, first, 100, "opened")
	_, two := nativeHost(t, second, 200, "opened")
	for _, target := range []struct {
		profile  string
		pid      int
		id       string
		received <-chan protocol.Request
	}{{first, 100, "a", one}, {second, 200, "b", two}} {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		client, err := Dial(ctx, target.profile, target.pid)
		if err != nil {
			t.Fatal(err)
		}
		want := packet(target.id)
		if err := client.Open(want); err != nil {
			t.Fatal(err)
		}
		_ = client.Close()
		cancel()
		got := <-target.received
		if got != want {
			t.Fatal("request changed in transport")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if client, err := Dial(ctx, first, 200); err == nil {
		_ = client.Close()
		t.Fatal("connected to wrong Firefox profile process")
	}
}
func TestNativeFailureClassification(t *testing.T) {
	for _, status := range []string{"rejected", "uncertain", "disconnect", "wrong-id"} {
		t.Run(status, func(t *testing.T) {
			runtimeDirectory(t)
			profile := filepath.Join(t.TempDir(), "profile")
			_, seen := nativeHost(t, profile, 100, status)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			client, err := Dial(ctx, profile, 100)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = client.Close() }()
			err = client.Open(packet("a"))
			if err == nil || MayHaveDelivered(err) != (status != "rejected") {
				t.Fatal("wrong certainty classification", err)
			}
			<-seen
			select {
			case <-seen:
				t.Fatal("transport retried a request")
			default:
			}
		})
	}
}
func TestHostLifetimeAndExclusiveSocket(t *testing.T) {
	runtimeDirectory(t)
	profile := filepath.Join(t.TempDir(), "profile")
	stop, _ := nativeHost(t, profile, 100, "opened")
	if listener, lock, err := listen(profile); err == nil {
		_ = listener.Close()
		_ = lock.Close()
		t.Fatal("two hosts own one profile")
	}
	path, err := socketPath(profile, false)
	if err != nil {
		t.Fatal(err)
	}
	stop()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatal("socket survived Firefox disconnect", err)
	}
	listener, lock, err := listen(profile)
	if err != nil {
		t.Fatal("could not restart host", err)
	}
	_ = listener.Close()
	_ = lock.Close()
}
func TestRuntimeDirectoryPermissions(t *testing.T) {
	runtimeDirectory(t)
	root := os.Getenv("XDG_RUNTIME_DIR")
	if err := os.Chmod(root, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := socketPath("/profile", true); err == nil {
		t.Fatal("accepted public runtime directory")
	}
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "niri-firefox-router")); err != nil {
		t.Fatal(err)
	}
	if _, err := socketPath("/profile", true); err == nil {
		t.Fatal("accepted symlink runtime directory")
	}
}
