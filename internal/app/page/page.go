package page // gofmt

import (
	"context"
	"crawler/internal/usecase"
	"fmt"
	"io"

	"github.com/PuerkitoBio/goquery"
	"go.uber.org/zap"
)

type page struct {
	doc    *goquery.Document
	logger *zap.Logger
}

func NewPage(raw io.Reader, logger *zap.Logger) (usecase.Page, error) {
	doc, err := goquery.NewDocumentFromReader(raw)
	if err != nil {
		logger.Error("new page error", zap.Error(err))
		return nil, err
	}
	logger.Debug("new page initialize")
	return &page{doc: doc, logger: logger}, nil
}

func (p *page) GetTitle(ctx context.Context) string {
	select {
	case <-ctx.Done():
		p.logger.Debug("context done in get title")

		return ""
	default:
		title := p.doc.Find("title").First().Text()
		logMsg := fmt.Sprintf("get title return title: %s", title)
		p.logger.Debug(logMsg)
		return title
	}
}

func (p *page) GetLinks(ctx context.Context) []string {
	select {
	case <-ctx.Done():
		p.logger.Debug("context done in get links")
		return nil
	default:
		var urls []string
		p.doc.Find("a").Each(func(_ int, s *goquery.Selection) {
			url, ok := s.Attr("href")
			p.logger.Debug("trying to get links")
			if ok {
				urls = append(urls, url)
				logMsg := fmt.Sprintf("write url %s in slice urls", url)
				p.logger.Debug(logMsg)
			}
		})
		return urls
	}
}
