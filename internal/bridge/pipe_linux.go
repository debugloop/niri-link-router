package bridge

import (
	"errors"
	"os"
	"syscall"
	"time"
)

// nativePipe lets Go interrupt native I/O through a duplicate descriptor.
// The duplicate shares nonblocking mode with the source. Do not use the source during the host session.
func nativePipe(source *os.File) (*os.File, error) {
	fd, err := syscall.Dup(int(source.Fd()))
	if err != nil {
		return nil, errors.New("cannot duplicate native messaging pipe")
	}
	syscall.CloseOnExec(fd)
	if err := syscall.SetNonblock(fd, true); err != nil {
		_ = syscall.Close(fd)
		return nil, errors.New("cannot make native messaging pipe nonblocking")
	}
	pipe := os.NewFile(uintptr(fd), source.Name())
	if err := pipe.SetDeadline(time.Time{}); err != nil {
		_ = pipe.Close()
		return nil, errors.New("native messaging pipe does not support deadlines")
	}
	return pipe, nil
}
