package page

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func ReadHTML() io.Reader {

	f, err := os.Open("../../../tests/test.html")
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
	l := zap.NewExample()
	html := ReadHTML()
	page, _ := NewPage(html, l)

	assert.NotNil(t, page, "fail to get new page")
}

func TestGetTitle(t *testing.T) {
	l := zap.NewExample()
	ctx := context.Background()
	html := ReadHTML()
	page, _ := NewPage(html, l)
	exp := "Document"
	got := page.GetTitle(ctx)
	assert.Equal(t, exp, got, "Test failed. Expect: %v, got: %v", exp, got)
}

func TestGetLinks(t *testing.T) {
	l := zap.NewExample()
	ctx := context.Background()
	html := ReadHTML()
	page, _ := NewPage(html, l)
	exp := []string{"http://example-1.com", "https://example-2.ru", "http://example-3.gov"}
	got := page.GetLinks(ctx)
	assert.Equal(t, exp, got, "Test failed. Expect: %v, got: %v", exp, got)
}
