package protocol

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const Version = 1
const HostName = "niri_firefox_router"
const MaxFrame = 64 * 1024
const MaxLifetime = 30 * time.Second

type Hello struct {
	Version    int    `json:"version"`
	Type       string `json:"type"`
	FirefoxPID int    `json:"firefox_pid,omitempty"`
}

type Request struct {
	Version  int    `json:"version"`
	Type     string `json:"type"`
	ID       string `json:"id"`
	WindowID int    `json:"window_id"`
	URL      string `json:"url"`
	Active   bool   `json:"active"`
	Expires  int64  `json:"expires"`
}

type Reply struct {
	Version int    `json:"version"`
	Type    string `json:"type"`
	ID      string `json:"id"`
	Status  string `json:"status"`
}

func ValidID(id string) bool {
	if len(id) != 32 {
		return false
	}
	for _, c := range id {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func (r Request) Validate(now time.Time) error {
	if r.Version != Version || r.Type != "open" || !ValidID(r.ID) || r.WindowID <= 0 || int64(r.WindowID) > 2147483647 {
		return errors.New("invalid native request")
	}
	if r.Expires <= now.UnixMilli() || r.Expires > now.Add(MaxLifetime).UnixMilli() {
		return errors.New("native request expired or exceeds deadline bound")
	}
	return ValidateURL(r.URL)
}

func (r Reply) Validate(id string) error {
	if r.Version != Version || r.Type != "result" || r.ID != id || !ValidID(r.ID) {
		return errors.New("unexpected native acknowledgement")
	}
	switch r.Status {
	case "opened", "rejected", "uncertain":
		return nil
	}
	return errors.New("invalid native acknowledgement status")
}

func ReadFrame(r io.Reader, value any) error {
	var header [4]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return err
	}
	size := binary.LittleEndian.Uint32(header[:])
	if size == 0 || size > MaxFrame {
		return errors.New("native message exceeds size bound")
	}
	data := make([]byte, size)
	if _, err := io.ReadFull(r, data); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return errors.New("invalid native message JSON")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return errors.New("native message must contain one JSON object")
	}
	return nil
}

func WriteFrame(w io.Writer, value any) error {
	data, err := json.Marshal(value)
	if err != nil || len(data) == 0 || len(data) > MaxFrame {
		return errors.New("cannot encode bounded native message")
	}
	var header [4]byte
	binary.LittleEndian.PutUint32(header[:], uint32(len(data)))
	for _, part := range [][]byte{header[:], data} {
		for len(part) > 0 {
			n, err := w.Write(part)
			if err != nil {
				return err
			}
			if n <= 0 {
				return io.ErrShortWrite
			}
			part = part[n:]
		}
	}
	return nil
}

func ValidateURL(raw string) error {
	if len(raw) == 0 || len(raw) > 8192 || !utf8.ValidString(raw) {
		return errors.New("invalid URL size or UTF-8")
	}
	for _, r := range raw {
		if unicode.IsSpace(r) || unicode.IsControl(r) || r == '\\' {
			return errors.New("URL contains whitespace, controls, or backslashes")
		}
	}
	for i := 0; i < len(raw); i++ {
		if raw[i] == '%' {
			if i+2 >= len(raw) {
				return errors.New("malformed URL escape")
			}
			if _, err := hex.DecodeString(raw[i+1 : i+3]); err != nil {
				return errors.New("malformed URL escape")
			}
			i += 2
		}
	}
	u, err := url.Parse(raw)
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.Opaque != "" || strings.Contains(u.Host, "\\") {
		return errors.New("URL must be absolute HTTP/HTTPS with a host and no credentials")
	}
	return nil
}
