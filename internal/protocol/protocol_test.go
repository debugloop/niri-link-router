package protocol

import (
	"bytes"
	"encoding/binary"
	"io"
	"strings"
	"testing"
	"time"
)

func request() Request {
	return Request{Version: 1, Type: "open", ID: strings.Repeat("a", 32), WindowID: 1, URL: "https://example.org/a+b?q=%25#fragment", Active: true, Expires: time.Now().Add(10 * time.Second).UnixMilli()}
}
func TestNativeFrameRoundTrip(t *testing.T) {
	var stream bytes.Buffer
	want := request()
	if err := WriteFrame(&stream, want); err != nil {
		t.Fatal(err)
	}
	if int(binary.LittleEndian.Uint32(stream.Bytes()[:4])) != stream.Len()-4 {
		t.Fatal("wrong native-endian frame size")
	}
	var got Request
	if err := ReadFrame(&stream, &got); err != nil || got != want {
		t.Fatal(got, err)
	}
	if err := got.Validate(time.Now()); err != nil {
		t.Fatal(err)
	}
}
func TestFrameBoundsAndShape(t *testing.T) {
	for _, data := range [][]byte{{}, {1, 0}, {0, 0, 0, 0}, {255, 255, 255, 255}, {5, 0, 0, 0, '{', '}'}} {
		if err := ReadFrame(bytes.NewReader(data), new(Request)); err == nil {
			t.Fatal("accepted invalid frame")
		}
	}
	for _, payload := range []string{`null`, `{"version":1,"unknown":true}`, `{} {}`} {
		var stream bytes.Buffer
		_ = binary.Write(&stream, binary.LittleEndian, uint32(len(payload)))
		stream.WriteString(payload)
		var value Request
		err := ReadFrame(&stream, &value)
		if err == nil {
			err = value.Validate(time.Now())
		}
		if err == nil {
			t.Fatal("accepted invalid message", payload)
		}
	}
	if err := WriteFrame(io.Discard, strings.Repeat("x", MaxFrame+1)); err == nil {
		t.Fatal("accepted oversized message")
	}
}
func TestRequestValidation(t *testing.T) {
	now := time.Now()
	changes := []func(*Request){func(r *Request) { r.Version = 0 }, func(r *Request) { r.Type = "pair" }, func(r *Request) { r.ID = "bad" }, func(r *Request) { r.WindowID = 0 }, func(r *Request) { r.Expires = now.UnixMilli() }, func(r *Request) { r.Expires = now.Add(time.Minute).UnixMilli() }, func(r *Request) { r.URL = "file:///tmp/private" }}
	for _, change := range changes {
		r := request()
		change(&r)
		if err := r.Validate(now); err == nil {
			t.Fatal("accepted invalid request")
		}
	}
}
func TestURLValidation(t *testing.T) {
	for _, url := range []string{"", "javascript:alert(1)", "file:///etc/passwd", "https://user:password@example.org/", "https://example.org/\n", "https://example.org/%xx", "https://example.org/\\evil"} {
		if err := ValidateURL(url); err == nil {
			t.Fatal("accepted invalid URL")
		}
	}
}
func TestReplies(t *testing.T) {
	id := strings.Repeat("a", 32)
	for _, status := range []string{"opened", "rejected", "uncertain"} {
		if err := (Reply{Version: 1, Type: "result", ID: id, Status: status}).Validate(id); err != nil {
			t.Fatal(err)
		}
	}
	if err := (Reply{Version: 1, Type: "result", ID: id, Status: "opened"}).Validate(strings.Repeat("b", 32)); err == nil {
		t.Fatal("accepted wrong acknowledgement")
	}
}
