package stores

import (
	"bytes"
	"io"
)

/*
TaskStore provides a streaming interface for task data using a connection.
It implements io.ReadWriteCloser to ensure compatibility with other streaming components.
*/
type TaskStore struct {
	conn Conn
	err  error
}

/*
NewTaskStore creates a new TaskStore instance with the given connection.
*/
func NewTaskStore(conn Conn) *TaskStore {
	return &TaskStore{
		conn: conn,
	}
}

/*
Read implements io.Reader by copying data from the connection to the provided buffer.
Returns the number of bytes read and any error encountered.
*/
func (ts *TaskStore) Read(p []byte) (n int, err error) {
	n64, err := io.Copy(bytes.NewBuffer(p), ts.conn)
	return int(n64), err
}

/*
Write implements io.Writer by copying data from the provided buffer to the connection.
Returns the number of bytes written and any error encountered.
*/
func (ts *TaskStore) Write(p []byte) (n int, err error) {
	n64, err := io.Copy(ts.conn, bytes.NewBuffer(p))
	return int(n64), err
}

/*
Close implements io.Closer by closing the underlying connection.
*/
func (ts *TaskStore) Close() error {
	return ts.conn.Close()
}
