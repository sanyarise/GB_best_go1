package crawler

import (
	"bufio"
	"bytes"
	"context"
	"crawler/internal/app/requester"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func ReadHTML() io.Reader {
	f, err := os.Open("../../../tests/test.html") // whitespace
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	h := bytes.Buffer{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		h.WriteString(sc.Text())
	}
	return &h
}

func TestNewCrawler(t *testing.T) {
	l := zap.NewExample()
	req := requester.NewRequester(10*time.Second, l, nil)
	cr := NewCrawler(req, l, 2)
	assert.NotNil(t, cr, "New crawler create fail")
}

type roundTripperFunc func(r *http.Request) (*http.Response, error)

func (rt roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return rt(r)
}

func TestCrawlerScan(t *testing.T) {
	url := "https://telegram.org"

	l := zap.NewExample()

	req := requester.NewRequester(3*time.Second, l, roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       ioutil.NopCloser(ReadHTML()),
		}, nil
	}))

	cr := NewCrawler(req, l, 3)
	ctx := context.Background()

	exp := []string{
		"url: " + url + ", title: Document",
		"url: http://example-1.com, title: Document",
		"url: https://example-2.ru, title: Document",
		"url: http://example-3.gov, title: Document",
	}

	go cr.Scan(ctx, url, 0)

	var ret []string
	var bool = true
	for bool {
		select {
		case <-time.After(10 * time.Second): // unconvert
			t.Log("scaning stop by timeout")
			bool = false
		case msg := <-cr.ChanResult():
			ret = append(ret, fmt.Sprintf("url: %s, title: %s", msg.URL, msg.Title))
		}
	}
	assert.ElementsMatch(t, ret, exp, "different results")
}
