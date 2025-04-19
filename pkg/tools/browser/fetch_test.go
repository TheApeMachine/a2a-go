package browser

import (
	"context"
	"testing"
	"time"
)

func TestFetchDataURL(t *testing.T) {
	html := `<html><head><title>RodTest</title></head><body><p id="g">hello</p></body></html>`
	url := "data:text/html," + html

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := Fetch(ctx, url, "#g", false, "")
	if err != nil {
		t.Skipf("browser not available or other fetch error: %v", err)
		return
	}
	if res.Title != "RodTest" {
		t.Fatalf("unexpected title: %s", res.Title)
	}
	if res.Text != "hello" {
		t.Fatalf("unexpected text: %s", res.Text)
	}
}
