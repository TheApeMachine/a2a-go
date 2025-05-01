package stores

import "io"

type Conn interface {
	io.ReadWriteCloser
}
