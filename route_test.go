package gx_test

import (
	"net/http"
	"testing"

	"github.com/alternayte/gx"
)

type fakeRoute struct {
	pattern string
}

func (f fakeRoute) Pattern() string                              { return f.pattern }
func (f fakeRoute) ServeHTTP(http.ResponseWriter, *http.Request) {}

func TestREQ_RTE_07_DuplicateStartupPanic(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("duplicate route did not panic at startup")
		}
	}()
	app := gx.New(gx.Config{})
	app.Group("/", fakeRoute{"GET /x"}, fakeRoute{"GET /x"})
}
