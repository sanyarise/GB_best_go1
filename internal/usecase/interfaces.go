package usecase // gofmt

import "context"

type CrawlResult struct {
	Err   error
	Title string
	URL   string // stylecheck
}

type Page interface {
	GetTitle(context.Context) string
	GetLinks(context.Context) []string
}

type Requester interface {
	Get(ctx context.Context, url string) (Page, error)
}

type Crawler interface {
	Scan(ctx context.Context, url string, depth int32)
	ChanResult() <-chan CrawlResult
	IncMaxDepth(int32)
}
