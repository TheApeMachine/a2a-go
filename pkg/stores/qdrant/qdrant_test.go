package qdrant

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestClientGet(t *testing.T) {
	Convey("Given a qdrant client and a test server", t, func() {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"result":{"id":"123","payload":{"content":"hello"}}}`)
		}))
		defer ts.Close()

		client := New(ts.URL, "mem")
		doc, err := client.Get(context.Background(), "123")

		Convey("Then the document should be parsed correctly", func() {
			So(err, ShouldBeNil)
			So(doc.ID, ShouldEqual, "123")
			So(doc.Content, ShouldEqual, "hello")
		})
	})
}

func TestClientSearch(t *testing.T) {
	Convey("Given a qdrant client and a test server for search", t, func() {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"result":[{"id":"1","payload":{"content":"a"}},{"id":"2","payload":{"content":"b"}}]}`)
		}))
		defer ts.Close()

		client := New(ts.URL, "mem")
		docs, err := client.Search(context.Background(), []float32{0.1}, 2)

		Convey("Then the search results should be returned", func() {
			So(err, ShouldBeNil)
			So(len(docs), ShouldEqual, 2)
			So(docs[0].Content, ShouldEqual, "a")
			So(docs[1].Content, ShouldEqual, "b")
		})
	})
}
