package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"niri-firefox-router/internal/routing"
)

// sourceWindow looks for the nearest ancestor that owns a Niri window.
// A process may own windows on several workspaces, so only use an
// unambiguous workspace match.
func sourceWindow(windows []routing.Window) *uint64 {
	for pid := os.Getppid(); pid > 1; {
		var match *routing.Window
		for i := range windows {
			w := &windows[i]
			if w.PID != pid {
				continue
			}
			if w.WorkspaceID == nil || (match != nil && *match.WorkspaceID != *w.WorkspaceID) {
				return nil
			}
			match = w
		}
		if match != nil {
			return &match.ID
		}
		stat, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
		if err != nil {
			break
		}
		// The process name is parenthesized and may itself contain spaces
		// and parentheses. The state and parent PID follow the final ')'.
		end := strings.LastIndexByte(string(stat), ')')
		fields := strings.Fields(string(stat[end+1:]))
		if end < 0 || len(fields) < 2 {
			break
		}
		parent, err := strconv.Atoi(fields[1])
		if err != nil || parent == pid {
			break
		}
		pid = parent
	}
	return nil
}
