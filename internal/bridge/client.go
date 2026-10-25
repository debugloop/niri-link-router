package bridge

import (
	"context"
	"errors"
	"net"
	"niri-firefox-router/internal/protocol"
	"time"
)

type DeliveryError struct{ Uncertain bool }

func (e *DeliveryError) Error() string {
	if e.Uncertain {
		return "Firefox delivery is uncertain; do not retry blindly"
	}
	return "Firefox native bridge is unavailable or rejected the request before delivery"
}
func MayHaveDelivered(err error) bool {
	var failure *DeliveryError
	return errors.As(err, &failure) && failure.Uncertain
}

type Client struct {
	conn     *net.UnixConn
	deadline time.Time
}

func Dial(ctx context.Context, profilePath string, firefoxPID int) (*Client, error) {
	path, err := socketPath(profilePath, false)
	if err != nil {
		return nil, &DeliveryError{}
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return nil, errors.New("native bridge needs a deadline")
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, "unix", path)
	if err != nil {
		return nil, &DeliveryError{}
	}
	unix, ok := conn.(*net.UnixConn)
	if !ok {
		_ = conn.Close()
		return nil, &DeliveryError{}
	}
	fail := func() (*Client, error) { _ = unix.Close(); return nil, &DeliveryError{} }
	if err := unix.SetDeadline(deadline); err != nil {
		return fail()
	}
	stop := context.AfterFunc(ctx, func() { _ = unix.Close() })
	defer stop()
	if err := peerUser(unix); err != nil {
		return fail()
	}
	var hello protocol.Hello
	if err := protocol.ReadFrame(unix, &hello); err != nil || hello.Version != protocol.Version || hello.Type != "ready" || hello.FirefoxPID != firefoxPID {
		return fail()
	}
	return &Client{conn: unix, deadline: deadline}, nil
}
func (c *Client) Close() error { return c.conn.Close() }
func (c *Client) Open(request protocol.Request) error {
	if err := request.Validate(time.Now()); err != nil {
		return &DeliveryError{}
	}
	if time.Now().After(c.deadline) {
		return &DeliveryError{}
	}
	if err := protocol.WriteFrame(c.conn, request); err != nil {
		return &DeliveryError{Uncertain: true}
	}
	var reply protocol.Reply
	if err := protocol.ReadFrame(c.conn, &reply); err != nil {
		return &DeliveryError{Uncertain: true}
	}
	if err := reply.Validate(request.ID); err != nil {
		return &DeliveryError{Uncertain: true}
	}
	switch reply.Status {
	case "opened":
		return nil
	case "rejected":
		return &DeliveryError{}
	default:
		return &DeliveryError{Uncertain: true}
	}
}
