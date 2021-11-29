package requester

import (
	"context"
	"crawler/internal/app/page"
	"crawler/internal/usecase"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
)

type requester struct {
	timeout time.Duration
	logger  *zap.Logger
	rt      http.RoundTripper
}

func NewRequester(timeout time.Duration, logger *zap.Logger, rt http.RoundTripper) requester {
	logger.Debug("new requester initialize")
	return requester{
		timeout: timeout,
		logger:  logger,
		rt:      rt,
	}
}

func (r requester) Get(ctx context.Context, url string) (usecase.Page, error) {
	select {
	case <-ctx.Done():
		r.logger.Debug("context done in get")
		return nil, nil
	default:
		cl := &http.Client{
			Timeout:   r.timeout,
			Transport: r.rt,
		}
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			logMsg := fmt.Sprintf("error by get new request, url: %s", url)
			r.logger.Error(logMsg)
			return nil, err
		}
		body, err := cl.Do(req)
		if err != nil {
			r.logger.Error("http.client error", zap.Error(err))
			return nil, err
		}
		defer body.Body.Close()
		page, err := page.NewPage(body.Body, r.logger)
		if err != nil {
			r.logger.Error("get new page error", zap.Error(err))
			return nil, err
		}
		return page, nil
	}
}
