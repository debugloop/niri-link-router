package bridge

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"os"
	"path/filepath"
	"syscall"
)

func privateDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return errors.New("native bridge runtime directory is unavailable")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.IsDir() || stat.Uid != uint32(os.Getuid()) || info.Mode().Perm()&0077 != 0 {
		return errors.New("native bridge runtime directory must be owned by this user with mode 0700")
	}
	return nil
}

func socketPath(profilePath string, create bool) (string, error) {
	root := os.Getenv("XDG_RUNTIME_DIR")
	if !filepath.IsAbs(root) {
		return "", errors.New("XDG_RUNTIME_DIR must be absolute")
	}
	if err := privateDirectory(root); err != nil {
		return "", err
	}
	dir := filepath.Join(root, "niri-firefox-router")
	if create {
		if err := os.Mkdir(dir, 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return "", err
		}
	}
	if err := privateDirectory(dir); err != nil {
		return "", err
	}
	if !filepath.IsAbs(profilePath) {
		return "", errors.New("native bridge needs an absolute profile path")
	}
	hash := sha256.Sum256([]byte(filepath.Clean(profilePath)))
	path := filepath.Join(dir, hex.EncodeToString(hash[:16])+".sock")
	if len(path) > 107 {
		return "", errors.New("native bridge socket path exceeds Linux limit")
	}
	return path, nil
}

func peerUser(conn *net.UnixConn) error {
	raw, err := conn.SyscallConn()
	if err != nil {
		return err
	}
	var peer *syscall.Ucred
	var failure error
	err = raw.Control(func(fd uintptr) {
		peer, failure = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	})
	if err != nil || failure != nil || peer == nil || peer.Uid != uint32(os.Getuid()) {
		return errors.New("native bridge peer belongs to another user")
	}
	return nil
}

func listen(profilePath string) (*net.UnixListener, *os.File, error) {
	path, err := socketPath(profilePath, true)
	if err != nil {
		return nil, nil, err
	}
	fd, err := syscall.Open(path+".lock", syscall.O_RDWR|syscall.O_CREAT|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0600)
	if err != nil {
		return nil, nil, errors.New("cannot open native bridge lock")
	}
	lock := os.NewFile(uintptr(fd), path+".lock")
	fail := func(err error) (*net.UnixListener, *os.File, error) { _ = lock.Close(); return nil, nil, err }
	info, err := lock.Stat()
	if err != nil {
		return fail(err)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() || stat.Uid != uint32(os.Getuid()) || info.Mode().Perm()&0077 != 0 {
		return fail(errors.New("unsafe native bridge lock"))
	}
	if err := syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return fail(errors.New("another native host already owns this profile"))
	}
	if info, err := os.Lstat(path); err == nil {
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok || info.Mode()&os.ModeSocket == 0 || stat.Uid != uint32(os.Getuid()) {
			return fail(errors.New("refuse to remove an unsafe native bridge socket"))
		}
		if err := os.Remove(path); err != nil {
			return fail(err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fail(err)
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return fail(err)
	}
	if err := os.Chmod(path, 0600); err != nil {
		_ = listener.Close()
		return fail(err)
	}
	return listener, lock, nil
}
