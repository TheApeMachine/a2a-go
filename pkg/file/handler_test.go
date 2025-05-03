package file

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestNewHandler(t *testing.T) {
	Convey("Given a base directory", t, func() {
		baseDir, err := os.MkdirTemp("", "file-test-*")
		So(err, ShouldBeNil)
		defer os.RemoveAll(baseDir)

		Convey("When creating a new handler", func() {
			handler, err := NewHandler(baseDir)

			Convey("It should initialize successfully", func() {
				So(err, ShouldBeNil)
				So(handler, ShouldNotBeNil)
				So(handler.handles, ShouldNotBeNil)
				So(len(handler.handles), ShouldEqual, 0)
			})
		})
	})
}

func TestOpen(t *testing.T) {
	Convey("Given a file handler", t, func() {
		baseDir, err := os.MkdirTemp("", "file-test-*")
		So(err, ShouldBeNil)
		defer os.RemoveAll(baseDir)

		handler, err := NewHandler(baseDir)
		So(err, ShouldBeNil)

		Convey("When opening a non-existent file", func() {
			handle, err := handler.Open(context.Background(), "test.txt", os.O_CREATE|os.O_RDWR)

			Convey("It should create and open the file", func() {
				So(err, ShouldBeNil)
				So(handle, ShouldEqual, "test.txt")
				So(handler.handles[handle], ShouldNotBeNil)
			})
		})

		Convey("When opening an existing file", func() {
			path := filepath.Join(baseDir, "test.txt")
			So(os.WriteFile(path, []byte("test"), 0644), ShouldBeNil)

			handle, err := handler.Open(context.Background(), "test.txt", os.O_RDONLY)

			Convey("It should open the file", func() {
				So(err, ShouldBeNil)
				So(handle, ShouldEqual, "test.txt")
				So(handler.handles[handle], ShouldNotBeNil)
			})
		})
	})
}

func TestReadWrite(t *testing.T) {
	Convey("Given a file handler with an open file", t, func() {
		baseDir, err := os.MkdirTemp("", "file-test-*")
		So(err, ShouldBeNil)
		defer os.RemoveAll(baseDir)

		handler, err := NewHandler(baseDir)
		So(err, ShouldBeNil)

		handle, err := handler.Open(context.Background(), "test.txt", os.O_CREATE|os.O_RDWR)
		So(err, ShouldBeNil)

		Convey("When writing to the file", func() {
			n, err := handler.Write(context.Background(), handle, []byte("test"))

			Convey("It should write successfully", func() {
				So(err, ShouldBeNil)
				So(n, ShouldEqual, 4)
			})
		})

		Convey("When reading from the file", func() {
			_, err := handler.Write(context.Background(), handle, []byte("test"))
			So(err, ShouldBeNil)

			// Seek back to the beginning
			_, err = handler.Seek(context.Background(), handle, 0, 0)
			So(err, ShouldBeNil)

			buf := make([]byte, 4)
			n, err := handler.Read(context.Background(), handle, buf)

			Convey("It should read successfully", func() {
				So(err, ShouldBeNil)
				So(n, ShouldEqual, 4)
				So(string(buf), ShouldEqual, "test")
			})
		})
	})
}

func TestClose(t *testing.T) {
	Convey("Given a file handler with an open file", t, func() {
		baseDir, err := os.MkdirTemp("", "file-test-*")
		So(err, ShouldBeNil)
		defer os.RemoveAll(baseDir)

		handler, err := NewHandler(baseDir)
		So(err, ShouldBeNil)

		handle, err := handler.Open(context.Background(), "test.txt", os.O_CREATE|os.O_RDWR)
		So(err, ShouldBeNil)

		Convey("When closing the file", func() {
			err := handler.Close(context.Background(), handle)

			Convey("It should close successfully", func() {
				So(err, ShouldBeNil)
				So(handler.handles[handle], ShouldBeNil)
			})
		})
	})
}

func TestBase64Conversion(t *testing.T) {
	Convey("Given a file handler", t, func() {
		baseDir, err := os.MkdirTemp("", "file-test-*")
		So(err, ShouldBeNil)
		defer os.RemoveAll(baseDir)

		handler, err := NewHandler(baseDir)
		So(err, ShouldBeNil)

		Convey("When converting a file to base64", func() {
			handle, err := handler.Open(context.Background(), "test.txt", os.O_CREATE|os.O_RDWR)
			So(err, ShouldBeNil)
			_, err = handler.Write(context.Background(), handle, []byte("test"))
			So(err, ShouldBeNil)

			// Seek back to the beginning
			_, err = handler.Seek(context.Background(), handle, 0, 0)
			So(err, ShouldBeNil)

			base64, err := handler.ToBase64(context.Background(), handle)

			Convey("It should convert successfully", func() {
				So(err, ShouldBeNil)
				So(base64, ShouldEqual, "dGVzdA==")
			})
		})

		Convey("When creating a file from base64", func() {
			err := handler.FromBase64(context.Background(), "test.txt", "dGVzdA==")

			Convey("It should create successfully", func() {
				So(err, ShouldBeNil)
				data, err := os.ReadFile(filepath.Join(baseDir, "test.txt"))
				So(err, ShouldBeNil)
				So(string(data), ShouldEqual, "test")
			})
		})
	})
}
