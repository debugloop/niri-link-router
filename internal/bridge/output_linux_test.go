package bridge

import (
	"context"
	"encoding/binary"
	"io"
	"niri-firefox-router/internal/protocol"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func pipePair(t *testing.T) (*os.File, *os.File) {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reader.Close(); _ = writer.Close() })
	return reader, writer
}

func TestNativeHostSubmissionDeadlineAndCancellation(t *testing.T) {
	for _, end := range []string{"write-deadline", "parent-cancel", "port-disconnect", "ack-deadline"} {
		t.Run(end, func(t *testing.T) {
			runtimeDirectory(t)
			input, browserWrite := pipePair(t)
			browserRead, output := pipePair(t)
			capacity, _, errno := syscall.Syscall(syscall.SYS_FCNTL, output.Fd(), syscall.F_SETPIPE_SZ, 8192)
			if errno != 0 {
				t.Fatal(errno)
			}
			if capacity != 8192 {
				t.Skip("the native request fits this system's minimum pipe capacity")
			}
			if err := browserRead.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			finished := make(chan error, 1)
			profile := filepath.Join(t.TempDir(), "profile")
			go func() {
				finished <- RunHost(ctx, Identity{FirefoxPID: 100, ProfilePath: profile}, input, output)
				close(finished)
			}()
			t.Cleanup(func() {
				cancel()
				_ = browserWrite.Close()
				_ = browserRead.Close()
				_ = output.Close()
				select {
				case err := <-finished:
					if err != nil {
						t.Log("native host stopped during cleanup:", err)
					}
				case <-time.After(time.Second):
					t.Error("native host did not stop after cleanup")
				}
			})
			if err := protocol.WriteFrame(browserWrite, protocol.Hello{Version: 1, Type: "hello"}); err != nil {
				t.Fatal(err)
			}
			var ready protocol.Hello
			if err := protocol.ReadFrame(browserRead, &ready); err != nil {
				t.Fatal(err)
			}
			clientCtx, stop := context.WithTimeout(context.Background(), 2*time.Second)
			defer stop()
			client, err := Dial(clientCtx, profile, 100)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = client.Close() }()
			request := packet("a")
			request.URL = "https://example.org/" + strings.Repeat("x", 8192-len("https://example.org/"))
			if end == "write-deadline" || end == "ack-deadline" {
				request.Expires = time.Now().Add(250 * time.Millisecond).UnixMilli()
			}
			result := make(chan error, 1)
			go func() { result <- client.Open(request) }()
			var header [4]byte
			if _, err := io.ReadFull(browserRead, header[:]); err != nil {
				t.Fatal(err)
			}
			size := binary.LittleEndian.Uint32(header[:])
			if size <= 8192 {
				t.Fatal("request does not fill the native pipe")
			}
			switch end {
			case "parent-cancel":
				cancel()
			case "port-disconnect":
				_ = browserWrite.Close()
			case "ack-deadline":
				if _, err := io.CopyN(io.Discard, browserRead, int64(size)); err != nil {
					t.Fatal(err)
				}
			}
			select {
			case err := <-finished:
				if (end == "write-deadline" || end == "ack-deadline") && err == nil {
					t.Fatal("native host did not report the expired submission")
				}
			case <-time.After(time.Second):
				t.Fatal("native host is still blocked after its request deadline or cancellation")
			}
			select {
			case err := <-result:
				if err == nil || !MayHaveDelivered(err) {
					t.Fatal("submission failure must remain uncertain", err)
				}
			case <-time.After(time.Second):
				t.Fatal("native client did not stop after its host exited")
			}
			path, err := socketPath(profile, false)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Lstat(path); !os.IsNotExist(err) {
				t.Fatal("failed host retained its socket", err)
			}
			_, received := nativeHost(t, profile, 100, "opened")
			reconnected, err := Dial(clientCtx, profile, 100)
			if err != nil {
				t.Fatal("failed host retained its profile lock", err)
			}
			defer func() { _ = reconnected.Close() }()
			next := packet("b")
			if err := reconnected.Open(next); err != nil {
				t.Fatal("new native host could not deliver", err)
			}
			if got := <-received; got != next {
				t.Fatal("reconnection replayed or changed a request")
			}
		})
	}
}

func TestNativeHostCancelsBlockedStartup(t *testing.T) {
	for _, stage := range []string{"input", "output"} {
		t.Run(stage, func(t *testing.T) {
			runtimeDirectory(t)
			input, browserWrite := pipePair(t)
			browserRead, output := pipePair(t)
			_ = input.Fd()
			if stage == "output" {
				capacity, _, errno := syscall.Syscall(syscall.SYS_FCNTL, output.Fd(), syscall.F_GETPIPE_SZ, 0)
				if errno != 0 {
					t.Fatal(errno)
				}
				if _, err := output.Write(make([]byte, capacity)); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			finished := make(chan error, 1)
			profile := filepath.Join(t.TempDir(), "profile")
			go func() {
				finished <- RunHost(ctx, Identity{FirefoxPID: 100, ProfilePath: profile}, input, output)
				close(finished)
			}()
			t.Cleanup(func() {
				cancel()
				_ = browserWrite.Close()
				_ = browserRead.Close()
				select {
				case err := <-finished:
					if err != nil {
						t.Log("native host stopped during startup cleanup:", err)
					}
				case <-time.After(time.Second):
					t.Error("native host did not stop after startup cleanup")
				}
			})
			if stage == "output" {
				if err := protocol.WriteFrame(browserWrite, protocol.Hello{Version: 1, Type: "hello"}); err != nil {
					t.Fatal(err)
				}
			}
			select {
			case err := <-finished:
				if err == nil {
					t.Fatal("blocked native startup unexpectedly succeeded")
				}
			case <-time.After(time.Second):
				t.Fatal("native startup ignored cancellation")
			}
		})
	}
}
