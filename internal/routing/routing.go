package routing

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type Workspace struct {
	ID      uint64  `json:"id"`
	Name    *string `json:"name"`
	Index   int     `json:"idx"`
	Output  *string `json:"output"`
	Focused bool    `json:"is_focused"`
	Active  bool    `json:"is_active"`
}
type Timestamp struct {
	Seconds uint64 `json:"secs"`
	Nanos   uint32 `json:"nanos"`
}
type Window struct {
	ID          uint64     `json:"id"`
	PID         int        `json:"pid"`
	Title       string     `json:"title"`
	AppID       string     `json:"app_id"`
	WorkspaceID *uint64    `json:"workspace_id"`
	Focused     bool       `json:"is_focused"`
	FocusTime   *Timestamp `json:"focus_timestamp"`
}
type Decision struct {
	Normal      bool   `json:"normal_launch"`
	NiriID      uint64 `json:"niri_window_id"`
	FirefoxID   int    `json:"firefox_window_id"`
	FirefoxPID  int    `json:"firefox_pid"`
	WorkspaceID uint64 `json:"workspace_id"`
	Output      string `json:"output"`
}

var prefix = regexp.MustCompile(`^\[nfr:([1-9][0-9]*)\] `)

func Firefox(app string) bool {
	switch app {
	case "firefox", "firefox-esr", "org.mozilla.firefox", "org.mozilla.firefox-esr":
		return true
	}
	return false
}
func FirefoxID(title string) (int, error) {
	m := prefix.FindStringSubmatch(title)
	if m == nil {
		return 0, errors.New("missing extension prefix")
	}
	id, err := strconv.ParseInt(m[1], 10, 32)
	if err != nil || id <= 0 || strings.Contains(title[len(m[0]):], "[nfr:") {
		return 0, errors.New("invalid extension prefix")
	}
	return int(id), nil
}
func output(w Workspace) string {
	if w.Output == nil {
		return ""
	}
	return *w.Output
}

type candidate struct {
	win Window
	ws  Workspace
}

func recent(a, b *Timestamp) int {
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		return -1
	}
	if b == nil {
		return 1
	}
	if a.Seconds > b.Seconds {
		return 1
	}
	if a.Seconds < b.Seconds {
		return -1
	}
	if a.Nanos > b.Nanos {
		return 1
	}
	if a.Nanos < b.Nanos {
		return -1
	}
	return 0
}
func Select(workspaces []Workspace, windows []Window, fallbackName string) (Decision, error) {
	return selectWindow(workspaces, windows, fallbackName, true, nil)
}

// SelectForOpen uses the launching window's workspace when it is known.
func SelectForOpen(workspaces []Workspace, windows []Window, fallbackName string, sourceID *uint64) (Decision, error) {
	return selectWindow(workspaces, windows, fallbackName, false, sourceID)
}

func selectWindow(workspaces []Workspace, windows []Window, fallbackName string, useFocus bool, sourceID *uint64) (Decision, error) {
	ws := make(map[uint64]Workspace)
	var current Workspace
	focused := 0
	var fallback *Workspace
	for _, w := range workspaces {
		if _, ok := ws[w.ID]; ok {
			return Decision{}, errors.New("duplicate workspace ID")
		}
		ws[w.ID] = w
		if w.Focused {
			current = w
			focused++
		}
		if w.Name != nil && *w.Name == fallbackName {
			if fallback != nil {
				return Decision{}, fmt.Errorf("ambiguous fallback workspace %q", fallbackName)
			}
			copy := w
			fallback = &copy
		}
	}
	if useFocus && (focused != 1 || output(current) == "") {
		return Decision{}, errors.New("niri must have one focused workspace on an output")
	}
	var localWorkspace *uint64
	if useFocus {
		localWorkspace = &current.ID
	} else if sourceID != nil {
		for _, w := range windows {
			if w.ID == *sourceID {
				localWorkspace = w.WorkspaceID
				break
			}
		}
	}
	var local, remote []candidate
	for _, w := range windows {
		if !Firefox(w.AppID) {
			continue
		}
		if w.WorkspaceID == nil {
			continue
		}
		space, ok := ws[*w.WorkspaceID]
		if !ok {
			return Decision{}, errors.New("window workspace disappeared during snapshot")
		}
		if localWorkspace != nil && space.ID == *localWorkspace {
			local = append(local, candidate{w, space})
		} else if fallback != nil && space.ID == fallback.ID {
			remote = append(remote, candidate{w, space})
		}
	}
	candidates := local
	if len(candidates) == 0 {
		candidates = remote
	}
	if len(candidates) == 0 {
		return Decision{Normal: true}, nil
	}
	sort.Slice(candidates, func(i, j int) bool {
		a, b := candidates[i].win, candidates[j].win
		if a.Focused != b.Focused {
			return a.Focused
		}
		if r := recent(a.FocusTime, b.FocusTime); r != 0 {
			return r > 0
		}
		return a.ID < b.ID
	})
	c := candidates[0]
	fid, _ := FirefoxID(c.win.Title)
	if fid != 0 {
		for _, other := range windows {
			if other.ID == c.win.ID || !Firefox(other.AppID) || other.PID != c.win.PID {
				continue
			}
			if id, _ := FirefoxID(other.Title); id == fid {
				return Decision{}, errors.New("duplicate Firefox window ID within one process")
			}
		}
	}
	return Decision{NiriID: c.win.ID, FirefoxID: fid, FirefoxPID: c.win.PID, WorkspaceID: c.ws.ID, Output: output(c.ws)}, nil
}
