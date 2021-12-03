package crawler

import (
	"context"
	"crawler/internal/usecase"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"go.uber.org/zap"
)

type crawler struct {
	r        usecase.Requester
	res      chan usecase.CrawlResult
	visited  map[string]struct{}
	mu       sync.RWMutex
	logger   *zap.Logger
	MaxDepth int32
}

func NewCrawler(r usecase.Requester, logger *zap.Logger, maxDepth int32) *crawler {
	return &crawler{
		r:        r,
		res:      make(chan usecase.CrawlResult),
		visited:  make(map[string]struct{}),
		mu:       sync.RWMutex{},
		logger:   logger,
		MaxDepth: maxDepth,
	}
}

func (c *crawler) Scan(ctx context.Context, url string, depth int32) {
	if depth >= atomic.LoadInt32(&c.MaxDepth) { //Проверяем то, что есть запас по глубине
		logMsg := fmt.Sprintf("actual depth: %d, maxdepth: %d", depth, c.MaxDepth)
		c.logger.Debug(logMsg)
		return
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		logMsg := fmt.Sprintf("%s is not a valid link", url)
		c.logger.Warn(logMsg)
		return
	}
	c.mu.RLock()
	_, ok := c.visited[url] //Проверяем, что мы ещё не смотрели эту страницу
	c.mu.RUnlock()
	if ok {
		logMsg := fmt.Sprintf("url %s is already in visited", url)
		c.logger.Debug(logMsg)
		return
	}
	select {
	case <-ctx.Done(): //Если контекст завершен - прекращаем выполнение
		c.logger.Debug("context done in crawler scan")
		return
	default:
		page, err := c.r.Get(ctx, url) //Запрашиваем страницу через Requester
		if err != nil {
			c.res <- usecase.CrawlResult{Err: err} //Записываем ошибку в канал
			c.logger.Error("error by get request", zap.Error(err))
			return
		}
		c.mu.Lock()
		c.visited[url] = struct{}{} //Помечаем страницу просмотренной
		logMsg := fmt.Sprintf("url %s moved in visited", url)
		c.logger.Debug(logMsg)
		c.mu.Unlock()
		c.res <- usecase.CrawlResult{ //Отправляем результаты в канал
			Title: page.GetTitle(ctx),
			URL:   url,
		}
		c.logger.Debug("sending results in channel")
		for _, link := range page.GetLinks(ctx) {
			go c.Scan(ctx, link, depth+1) //На все полученные ссылки запускаем новую рутину сборки
			logMsg := fmt.Sprintf("starting new example crawler scan, actual depth: %d", depth)
			c.logger.Debug(logMsg)
		}
	}
}

func (c *crawler) IncMaxDepth(delta int32) {
	atomic.AddInt32(&c.MaxDepth, delta)
	logMsg := fmt.Sprintf("new maxDepth: %d", c.MaxDepth)
	c.logger.Debug(logMsg)
}

func (c *crawler) ChanResult() <-chan usecase.CrawlResult {
	c.logger.Debug("chanresult is working")
	return c.res
}
