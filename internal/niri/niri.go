package niri

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"niri-firefox-router/internal/routing"
	"reflect"
	"sort"
	"time"
)

const MaxReply = 4 * 1024 * 1024

type Client struct {
	Socket  string
	Timeout time.Duration
}

func (c Client) Request(request any, result any) error {
	if c.Socket == "" {
		return errors.New("NIRI_SOCKET is not set")
	}
	if c.Timeout <= 0 {
		return errors.New("timeout must be positive")
	}
	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "unix", c.Socket)
	if err != nil {
		return fmt.Errorf("connect niri: %w", err)
	}
	defer func() { _ = conn.Close() }()
	deadline, _ := ctx.Deadline()
	if err = conn.SetDeadline(deadline); err != nil {
		return err
	}
	if err = json.NewEncoder(conn).Encode(request); err != nil {
		return fmt.Errorf("write niri: %w", err)
	}
	reader := bufio.NewReader(io.LimitReader(conn, MaxReply+1))
	line, err := reader.ReadBytes('\n')
	if len(line) > MaxReply {
		return errors.New("niri reply exceeds size bound")
	}
	if err != nil {
		return fmt.Errorf("incomplete niri reply: %w", err)
	}
	var reply map[string]json.RawMessage
	if err = json.Unmarshal(line, &reply); err != nil {
		return errors.New("invalid niri reply")
	}
	if e, ok := reply["Err"]; ok {
		var msg string
		_ = json.Unmarshal(e, &msg)
		return fmt.Errorf("niri error: %s", msg)
	}
	ok, exists := reply["Ok"]
	if !exists || len(reply) != 1 {
		return errors.New("missing niri Ok reply")
	}
	if err = json.Unmarshal(ok, result); err != nil {
		return errors.New("unexpected niri response")
	}
	return nil
}
func (c Client) workspaces() ([]routing.Workspace, error) {
	var r struct {
		Workspaces *[]routing.Workspace `json:"Workspaces"`
	}
	err := c.Request("Workspaces", &r)
	if err != nil {
		return nil, err
	}
	if r.Workspaces == nil {
		return nil, errors.New("missing Workspaces response")
	}
	sort.Slice(*r.Workspaces, func(i, j int) bool { return (*r.Workspaces)[i].ID < (*r.Workspaces)[j].ID })
	return *r.Workspaces, nil
}
func (c Client) Snapshot() ([]routing.Workspace, []routing.Window, error) {
	for attempt := 0; attempt < 2; attempt++ {
		before, err := c.workspaces()
		if err != nil {
			return nil, nil, err
		}
		var r struct {
			Windows *[]routing.Window `json:"Windows"`
		}
		if err = c.Request("Windows", &r); err != nil {
			return nil, nil, err
		}
		if r.Windows == nil {
			return nil, nil, errors.New("missing Windows response")
		}
		after, err := c.workspaces()
		if err != nil {
			return nil, nil, err
		}
		if reflect.DeepEqual(before, after) {
			return after, *r.Windows, nil
		}
	}
	return nil, nil, errors.New("niri workspace snapshot changed twice. Retry the command")
}
func (c Client) Focus(id uint64) error {
	return c.windowAction("FocusWindow", id)
}

func (c Client) SetUrgent(id uint64) error {
	return c.windowAction("SetWindowUrgent", id)
}

func (c Client) windowAction(action string, id uint64) error {
	var response string
	err := c.Request(map[string]any{"Action": map[string]any{action: map[string]uint64{"id": id}}}, &response)
	if err != nil {
		return err
	}
	if response != "Handled" {
		return errors.New("unexpected niri action response")
	}
	return nil
}
