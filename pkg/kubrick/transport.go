package kubrick

import (
	"io"
	"os"
)

// Transport represents a bidirectional communication channel for terminal I/O
type Transport interface {
	io.ReadWriteCloser
	GetSize() (width int, height int, err error)
	SetRawMode() error
	RestoreMode() error
}

// LocalTransport implements Transport for local terminal I/O
type LocalTransport struct {
	in      *os.File
	out     *os.File
	oldMode interface{}
}

// NewLocalTransport creates a new local terminal transport
func NewLocalTransport() *LocalTransport {
	return &LocalTransport{
		in:  os.Stdin,
		out: os.Stdout,
	}
}

func (t *LocalTransport) Read(p []byte) (n int, err error) {
	return t.in.Read(p)
}

func (t *LocalTransport) Write(p []byte) (n int, err error) {
	return t.out.Write(p)
}

func (t *LocalTransport) Close() error {
	t.RestoreMode()
	return nil
}

func (t *LocalTransport) GetSize() (width int, height int, err error) {
	return GetTerminalSize(int(t.out.Fd()))
}

func (t *LocalTransport) SetRawMode() error {
	mode, err := MakeRawMode(int(t.in.Fd()))
	if err != nil {
		return err
	}
	t.oldMode = mode
	return nil
}

func (t *LocalTransport) RestoreMode() error {
	if t.oldMode != nil {
		return RestoreMode(int(t.in.Fd()), t.oldMode)
	}
	return nil
}

// StreamTransport implements Transport for any io.ReadWriteCloser
type StreamTransport struct {
	rw            io.ReadWriteCloser
	width, height int
}

// NewStreamTransport creates a transport from any io.ReadWriteCloser
func NewStreamTransport(rw io.ReadWriteCloser, width, height int) *StreamTransport {
	return &StreamTransport{
		rw:     rw,
		width:  width,
		height: height,
	}
}

func (t *StreamTransport) Read(p []byte) (n int, err error) {
	return t.rw.Read(p)
}

func (t *StreamTransport) Write(p []byte) (n int, err error) {
	return t.rw.Write(p)
}

func (t *StreamTransport) Close() error {
	return t.rw.Close()
}

func (t *StreamTransport) GetSize() (width int, height int, err error) {
	return t.width, t.height, nil
}

func (t *StreamTransport) SetRawMode() error {
	// No-op for stream transport
	return nil
}

func (t *StreamTransport) RestoreMode() error {
	// No-op for stream transport
	return nil
}