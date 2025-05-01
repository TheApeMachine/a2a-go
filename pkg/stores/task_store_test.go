package stores

import (
	"bytes"
	"io"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

type MockConn struct {
	io.ReadWriteCloser
	payload *bytes.Buffer
}

func NewMockConn(payload []byte) *MockConn {
	return &MockConn{payload: bytes.NewBuffer(payload)}
}

func (mc *MockConn) Read(p []byte) (n int, err error) {
	return mc.payload.Read(p)
}

func (mc *MockConn) Write(p []byte) (n int, err error) {
	return mc.payload.Write(p)
}

func (mc *MockConn) Close() error {
	mc.payload.Reset()
	return nil
}

func TestNewTaskStore(t *testing.T) {
	Convey("Given a Conn", t, func() {
		conn := NewMockConn([]byte("test"))

		Convey("When instantiating a new TaskStore", func() {
			ts := NewTaskStore(conn)

			Convey("Then the TaskStore should be created", func() {
				So(ts, ShouldNotBeNil)
				So(ts.conn, ShouldEqual, conn)
				So(ts.err, ShouldBeNil)
			})
		})
	})
}

func TestRead(t *testing.T) {
	Convey("Given a TaskStore", t, func() {
		conn := NewMockConn([]byte("test"))
		ts := NewTaskStore(conn)

		Convey("When Reading from Conn", func() {
			buf := bytes.NewBuffer([]byte{})
			n, err := io.Copy(buf, ts.conn)

			Convey("Then the payload should be transferred", func() {
				So(err, ShouldBeNil)
				So(n, ShouldEqual, buf.Len())
				So(buf.String(), ShouldEqual, "test")
			})
		})

		Convey("When closing the Conn", func() {
			err := ts.conn.Close()

			Convey("Then the TaskStore should be reset", func() {
				So(err, ShouldBeNil)
				So(conn.payload.String(), ShouldEqual, "")
			})
		})
	})
}

func TestWrite(t *testing.T) {
	Convey("Given a TaskStore", t, func() {
		conn := NewMockConn(nil)
		ts := NewTaskStore(conn)

		Convey("When Writing to Conn", func() {
			payload := []byte("toast")
			n, err := io.Copy(ts.conn, bytes.NewBuffer(payload))

			Convey("Then the payload should be transferred", func() {
				So(err, ShouldBeNil)
				So(n, ShouldEqual, len(payload))
				So(conn.payload.String(), ShouldEqual, string(payload))
			})
		})

		Convey("When closing the Conn", func() {
			err := ts.conn.Close()

			Convey("Then the TaskStore should be reset", func() {
				So(err, ShouldBeNil)
				So(conn.payload.Len(), ShouldEqual, 0)
			})
		})
	})
}

func BenchmarkRead(b *testing.B) {
	conn := NewMockConn([]byte("test"))
	ts := NewTaskStore(conn)

	for i := 0; i < b.N; i++ {
		ts.Read(nil)
	}
}

func BenchmarkWrite(b *testing.B) {
	conn := NewMockConn(nil)
	ts := NewTaskStore(conn)

	for i := 0; i < b.N; i++ {
		ts.Write(nil)
	}
}
