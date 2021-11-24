package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func readHTML() io.Reader {

	f, err := os.Open("./test.html")
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

func TestNewPage(t *testing.T) {
	html := readHTML()
	page, _ := NewPage(html)
	
	assert.NotNil(t, page, "fail to get new page")	
}

func TestGetTitle(t *testing.T) {
	ctx := context.Background()
	html := readHTML()
	page, _ := NewPage(html)
	exp := "Document"
	got := page.GetTitle(ctx)
	assert.Equal(t, exp, got, "Test failed. Expect: %v, got: %v", exp, got)
}

func TestGetLinks(t *testing.T) {
	ctx := context.Background()
	html := readHTML()
	page, _ := NewPage(html)
	exp := []string{"http://example-1.com", "https://example-2.ru", "http://example-3.gov"}
	got := page.GetLinks(ctx)
	assert.Equal(t, exp, got, "Test failed. Expect: %v, got: %v", exp, got)
}

func TestNewRequester(t *testing.T) {
	req := NewRequester(3 * time.Second, nil)
	assert.NotNil(t, req, "Create new requester failed")
}

func TestReqGet(t *testing.T) {
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

	req := NewRequester(10*time.Second, nil)

	page, err := req.Get(ctx, url)
	if err != nil {
		t.Error("Get page is fail")
	}

	assert.NotNil(t, page, "Nil page")

}

func TestNewCrawler(t *testing.T) {
	req := NewRequester(10*time.Second, nil)
	cr := NewCrawler(req, 2)
	assert.NotNil(t, cr, "New crawler create fail")
}


type roundTripperFunc func(r *http.Request) (*http.Response, error)

func (rt roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return rt(r)
}

func TestCrawlerScan(t *testing.T) {
	url := "https://telegram.org"

	req := NewRequester(3*time.Second, roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       ioutil.NopCloser(readHTML()),
		}, nil
	}))

	cr := NewCrawler(req, 5)
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
		case <-time.After(time.Duration(10 * time.Second)):
			t.Log("scaning stop by timeout")
			bool = false
		case msg := <-cr.ChanResult():
			ret = append(ret, fmt.Sprintf("url: %s, title: %s", msg.Url, msg.Title))
		}
	}
	assert.ElementsMatch(t, ret, exp, "different results")
}
