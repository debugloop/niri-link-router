package bridge

import (
	"context"
	"errors"
	"net"
	"niri-firefox-router/internal/protocol"
	"os"
	"sync"
	"time"
)

type Identity struct {
	FirefoxPID  int
	ProfilePath string
}
type job struct {
	request protocol.Request
	reply   chan protocol.Reply
}

func RunHost(parent context.Context, identity Identity, input, output *os.File) error {
	if identity.FirefoxPID <= 0 {
		return errors.New("native host has no Firefox owner")
	}
	input, err := nativePipe(input)
	if err != nil {
		return err
	}
	defer func() { _ = input.Close() }()
	output, err = nativePipe(output)
	if err != nil {
		return err
	}
	defer func() { _ = output.Close() }()
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	stopIO := context.AfterFunc(ctx, func() { _ = input.Close(); _ = output.Close() })
	defer stopIO()
	handshakeDeadline := time.Now().Add(5 * time.Second)
	handshake := time.AfterFunc(time.Until(handshakeDeadline), cancel)
	defer handshake.Stop()
	var hello protocol.Hello
	err = protocol.ReadFrame(input, &hello)
	if err != nil || hello.Version != protocol.Version || hello.Type != "hello" || hello.FirefoxPID != 0 {
		return errors.New("invalid native bridge handshake")
	}
	listener, lock, err := listen(identity.ProfilePath)
	if err != nil {
		return err
	}
	defer func() { _ = listener.Close(); _ = lock.Close() }()
	stopListener := context.AfterFunc(ctx, func() { _ = listener.Close() })
	defer stopListener()
	if err := output.SetWriteDeadline(handshakeDeadline); err != nil {
		return errors.New("cannot set native bridge startup deadline")
	}
	if err := protocol.WriteFrame(output, protocol.Hello{Version: protocol.Version, Type: "ready", FirefoxPID: identity.FirefoxPID}); err != nil {
		return errors.New("cannot acknowledge native bridge startup")
	}
	handshake.Stop()
	replies := make(chan protocol.Reply, 1)
	go func() {
		defer cancel()
		for {
			var reply protocol.Reply
			if err := protocol.ReadFrame(input, &reply); err != nil {
				return
			}
			select {
			case replies <- reply:
			case <-ctx.Done():
				return
			}
		}
	}()
	jobs := make(chan job)
	var clients sync.WaitGroup
	slots := make(chan struct{}, 32)
	acceptDone := make(chan struct{})
	go func() {
		defer close(acceptDone)
		for {
			conn, err := listener.AcceptUnix()
			if err != nil {
				cancel()
				return
			}
			select {
			case slots <- struct{}{}:
				clients.Add(1)
				go func() {
					defer clients.Done()
					defer func() { <-slots }()
					serveClient(ctx, conn, identity.FirefoxPID, jobs)
				}()
			default:
				_ = conn.Close()
			}
		}
	}()
	defer func() { cancel(); _ = listener.Close(); <-acceptDone; clients.Wait() }()
	for {
		select {
		case <-ctx.Done():
			return nil
		case current := <-jobs:
			rejected := protocol.Reply{Version: protocol.Version, Type: "result", ID: current.request.ID, Status: "rejected"}
			if err := current.request.Validate(time.Now()); err != nil {
				current.reply <- rejected
				continue
			}
			expires := time.UnixMilli(current.request.Expires)
			if err := output.SetWriteDeadline(expires); err != nil {
				return errors.New("cannot set native bridge submission deadline")
			}
			if err := protocol.WriteFrame(output, current.request); err != nil {
				return errors.New("native bridge submission failed or timed out. Delivery is uncertain")
			}
			deadline := time.NewTimer(time.Until(expires))
			select {
			case reply := <-replies:
				deadline.Stop()
				if err := reply.Validate(current.request.ID); err != nil {
					return errors.New("native bridge acknowledgement does not match request")
				}
				current.reply <- reply
			case <-deadline.C:
				return errors.New("native bridge acknowledgement timed out; delivery is uncertain")
			case <-ctx.Done():
				deadline.Stop()
				return nil
			}
		}
	}
}

func serveClient(ctx context.Context, conn *net.UnixConn, pid int, jobs chan<- job) {
	defer func() { _ = conn.Close() }()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	if err := conn.SetDeadline(time.Now().Add(protocol.MaxLifetime)); err != nil {
		return
	}
	if err := peerUser(conn); err != nil {
		return
	}
	if err := protocol.WriteFrame(conn, protocol.Hello{Version: protocol.Version, Type: "ready", FirefoxPID: pid}); err != nil {
		return
	}
	var request protocol.Request
	if err := protocol.ReadFrame(conn, &request); err != nil {
		return
	}
	if err := request.Validate(time.Now()); err != nil {
		return
	}
	deadline := time.UnixMilli(request.Expires)
	if err := conn.SetDeadline(deadline); err != nil {
		return
	}
	clientContext, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	current := job{request: request, reply: make(chan protocol.Reply, 1)}
	select {
	case jobs <- current:
	case <-clientContext.Done():
		return
	}
	select {
	case reply := <-current.reply:
		_ = protocol.WriteFrame(conn, reply)
	case <-clientContext.Done():
		return
	}
}
