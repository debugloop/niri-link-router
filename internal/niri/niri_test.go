package niri

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func server(t *testing.T, handler func(net.Conn)) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "niri.sock")
	l, err := net.Listen("unix", p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func() { defer func() { _ = c.Close() }(); handler(c) }()
		}
	}()
	return p
}
func TestFraming(t *testing.T) {
	p := server(t, func(c net.Conn) {
		line, _ := bufio.NewReader(c).ReadString('\n')
		if line != "\"Workspaces\"\n" {
			return
		}
		_, _ = c.Write([]byte("{\"Ok\":{\"Workspaces\":[]}}\n"))
	})
	c := Client{p, time.Second}
	ws, err := c.workspaces()
	if err != nil || len(ws) != 0 {
		t.Fatal(ws, err)
	}
}
func TestReplyFailures(t *testing.T) {
	for _, r := range []string{"{\"Err\":\"no\"}\n", "{\"Ok\":{}", "{}\n", "not JSON\n", "{\"Ok\":\"Handled\"}\n", strings.Repeat("x", MaxReply+2)} {
		t.Run(r[:min(len(r), 20)], func(t *testing.T) {
			p := server(t, func(c net.Conn) { _, _ = bufio.NewReader(c).ReadString('\n'); _, _ = c.Write([]byte(r)) })
			_, err := (Client{p, time.Second}).workspaces()
			if err == nil {
				t.Fatal("accepted invalid response")
			}
		})
	}
}
func TestDeadline(t *testing.T) {
	done := make(chan struct{})
	defer close(done)
	p := server(t, func(c net.Conn) { _, _ = bufio.NewReader(c).ReadString('\n'); <-done })
	start := time.Now()
	_, err := (Client{p, 30 * time.Millisecond}).workspaces()
	if err == nil || time.Since(start) > time.Second {
		t.Fatal("deadline not enforced", err)
	}
}
func TestSnapshotRetry(t *testing.T) {
	for _, always := range []bool{false, true} {
		t.Run(map[bool]string{true: "twice", false: "once"}[always], func(t *testing.T) {
			var calls atomic.Int32
			var queries atomic.Int32
			p := server(t, func(c net.Conn) {
				line, _ := bufio.NewReader(c).ReadString('\n')
				queries.Add(1)
				if line == "\"Windows\"\n" {
					_, _ = c.Write([]byte("{\"Ok\":{\"Windows\":[]}}\n"))
					return
				}
				n := calls.Add(1)
				idx := 1
				if always {
					idx = int(n)
				} else if n > 1 {
					idx = 2
				}
				_ = json.NewEncoder(c).Encode(map[string]any{"Ok": map[string]any{"Workspaces": []any{map[string]any{"id": 0, "idx": idx}}}})
			})
			_, _, err := (Client{p, time.Second}).Snapshot()
			if (err != nil) != always || queries.Load() != 6 {
				t.Fatalf("err=%v queries=%d", err, queries.Load())
			}
		})
	}
}
func TestFocusID(t *testing.T) {
	received := make(chan string, 1)
	p := server(t, func(c net.Conn) {
		line, _ := bufio.NewReader(c).ReadString('\n')
		received <- line
		_, _ = c.Write([]byte("{\"Ok\":\"Handled\"}\n"))
	})
	if err := (Client{p, time.Second}).Focus(999); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(<-received, `"id":999`) {
		t.Fatal("wrong compositor ID")
	}
}
func TestConnectionFailures(t *testing.T) {
	for _, p := range []string{"", filepath.Join(os.TempDir(), "nfr-absent-socket")} {
		var r any
		if (Client{p, time.Millisecond}).Request("Windows", &r) == nil {
			t.Fatal("accepted absent socket")
		}
	}
}
