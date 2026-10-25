package profile

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const bound = 1024 * 1024

type Resolver struct {
	Proc  string
	Roots []string
}

func Default() Resolver {
	home, _ := os.UserHomeDir()
	config := os.Getenv("XDG_CONFIG_HOME")
	if config == "" {
		config = filepath.Join(home, ".config")
	}
	return Resolver{Proc: "/proc", Roots: []string{filepath.Join(config, "mozilla/firefox"), filepath.Join(home, ".mozilla/firefox")}}
}
func readBounded(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, bound+1))
	if err != nil {
		return nil, err
	}
	if len(data) > bound {
		return nil, errors.New("profile metadata exceeds size bound")
	}
	return data, nil
}
func registered(root string) ([]string, error) {
	data, err := readBounded(filepath.Join(root, "profiles.ini"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var paths []string
	var section string
	fields := map[string]string{}
	flush := func() {
		if !strings.HasPrefix(section, "Profile") || fields["Path"] == "" {
			return
		}
		p := fields["Path"]
		if fields["IsRelative"] == "1" {
			p = filepath.Join(root, p)
		}
		if filepath.IsAbs(p) {
			paths = append(paths, filepath.Clean(p))
		}
	}
	scan := bufio.NewScanner(strings.NewReader(string(data)))
	for scan.Scan() {
		line := strings.TrimSpace(scan.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			flush()
			section = line[1 : len(line)-1]
			fields = map[string]string{}
			continue
		}
		if k, v, ok := strings.Cut(line, "="); ok {
			fields[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	flush()
	return paths, scan.Err()
}
func lockedBy(path string, pid int) bool {
	target, err := os.Readlink(filepath.Join(path, "lock"))
	if err != nil {
		return false
	}
	i := strings.LastIndex(target, ":")
	if i < 0 {
		return false
	}
	owner, err := strconv.Atoi(strings.TrimPrefix(target[i+1:], "+"))
	return err == nil && owner == pid
}
func (r Resolver) Resolve(pid int) (string, error) {
	if pid <= 0 {
		return "", errors.New("niri did not supply the Firefox process ID")
	}
	proc := filepath.Join(r.Proc, strconv.Itoa(pid))
	info, err := os.Stat(proc)
	if err != nil {
		return "", errors.New("the Firefox process disappeared")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Getuid()) {
		return "", errors.New("the Firefox process belongs to another user")
	}
	command, err := readBounded(filepath.Join(proc, "cmdline"))
	if err != nil {
		return "", err
	}
	args := strings.Split(strings.TrimSuffix(string(command), "\x00"), "\x00")
	if len(args) == 0 || !strings.Contains(strings.ToLower(filepath.Base(args[0])), "firefox") {
		return "", errors.New("destination process is not Firefox")
	}
	var candidates []string
	for i := 1; i < len(args); i++ {
		arg := strings.ToLower(args[i])
		var path string
		if arg == "--profile" || arg == "-profile" {
			if i+1 >= len(args) {
				return "", errors.New("missing Firefox profile argument")
			}
			i++
			path = args[i]
		} else if strings.HasPrefix(arg, "--profile=") {
			path = args[i][len("--profile="):]
		} else if strings.HasPrefix(arg, "-profile=") {
			path = args[i][len("-profile="):]
		}
		if path != "" {
			if !filepath.IsAbs(path) {
				cwd, err := os.Readlink(filepath.Join(proc, "cwd"))
				if err != nil {
					return "", err
				}
				path = filepath.Join(cwd, path)
			}
			candidates = append(candidates, path)
		}
	}
	for _, root := range r.Roots {
		paths, err := registered(root)
		if err != nil {
			return "", fmt.Errorf("read Firefox profile registry: %w", err)
		}
		candidates = append(candidates, paths...)
	}
	matches := map[string]bool{}
	for _, path := range candidates {
		if !lockedBy(path, pid) {
			continue
		}
		canonical, err := filepath.EvalSymlinks(path)
		if err != nil {
			return "", err
		}
		matches[canonical] = true
	}
	if len(matches) != 1 {
		return "", errors.New("cannot identify one locked Firefox profile for the selected process")
	}
	for path := range matches {
		return path, nil
	}
	panic("unreachable")
}

func (r Resolver) BrowserAncestor(pid int) (int, string, error) {
	seen := make(map[int]bool)
	for step := 0; step < 32 && pid > 1; step++ {
		if seen[pid] {
			break
		}
		seen[pid] = true
		if path, err := r.Resolve(pid); err == nil {
			return pid, path, nil
		}
		proc := filepath.Join(r.Proc, strconv.Itoa(pid))
		info, err := os.Stat(proc)
		if err != nil {
			break
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok || stat.Uid != uint32(os.Getuid()) {
			break
		}
		data, err := readBounded(filepath.Join(proc, "status"))
		if err != nil {
			break
		}
		parent := 0
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "PPid:") {
				parent, _ = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "PPid:")))
				break
			}
		}
		pid = parent
	}
	return 0, "", errors.New("native host must be a child of this user's locked Firefox profile")
}
