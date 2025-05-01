package transport

import (
	"bytes"
	"encoding/json"
	"io"
	"sync"
)

/*
Stream provides a generic object that can be adopted by any other object
that needs to be streamed. The adopting object must be JSON serializable.
*/
type Stream[T any] struct {
	obj    *T
	buffer *bytes.Buffer
	mu     sync.Mutex
	closed bool
}

/*
NewStream creates a new Stream wrapper for a given JSON-serializable object.
*/
func NewStream[T any](obj *T) *Stream[T] {
	return &Stream[T]{obj: obj, buffer: bytes.NewBuffer(nil)}
}

/*
Read encodes the wrapped object into JSON on-demand and streams it out.
*/
func (stream *Stream[T]) Read(p []byte) (n int, err error) {
	stream.mu.Lock()
	defer stream.mu.Unlock()

	if stream.closed {
		return 0, io.EOF
	}

	if stream.buffer.Len() == 0 {
		encoder := json.NewEncoder(stream.buffer)

		if err := encoder.Encode(stream.obj); err != nil {
			return 0, err
		}
	}

	if n, err = stream.buffer.Read(p); err == io.EOF {
		stream.closed = true
	}

	return n, err
}

/*
Write decodes incoming JSON data into the wrapped object.
*/
func (stream *Stream[T]) Write(p []byte) (n int, err error) {
	stream.mu.Lock()
	defer stream.mu.Unlock()

	if stream.closed {
		return 0, io.ErrClosedPipe
	}

	decoder := json.NewDecoder(bytes.NewReader(p))

	if err = decoder.Decode(stream.obj); err != nil {
		return 0, err
	}

	return len(p), nil
}

/*
Close sets the object as closed (idempotent).
*/
func (stream *Stream[T]) Close() error {
	stream.mu.Lock()
	defer stream.mu.Unlock()

	stream.closed = true
	return nil
}
