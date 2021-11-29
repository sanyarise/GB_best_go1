package requester

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestNewRequester(t *testing.T) {
	l := zap.NewExample()
	req := NewRequester(3*time.Second, l, nil)
	assert.NotNil(t, req, "Create new requester failed")
}

func TestReqGet(t *testing.T) {
	l := zap.NewExample()
	addr := "localhost:8000"
	url := "http://localhost:8000"
	s := &http.Server{Addr: addr, Handler: nil}
	go func() {
		err := s.ListenAndServe()
		if err != nil {
			t.Error("Start server fail")
			return
		}
	}()
	time.Sleep(3 * time.Second)

	ctx := context.Background()

	req := NewRequester(10*time.Second, l, nil)

	page, err := req.Get(ctx, url)
	if err != nil {
		t.Error("Get page is fail")
	}

	assert.NotNil(t, page, "Nil page")

}
