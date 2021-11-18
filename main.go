package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"

	"sync/atomic"
	"syscall"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/PuerkitoBio/goquery"
)

type CrawlResult struct {
	Err   error
	Title string
	Url   string
}

type Page interface {
	GetTitle(context.Context) string
	GetLinks(context.Context) []string
}

type page struct {
	doc    *goquery.Document
	logger *zap.Logger
}

func NewPage(raw io.Reader, logger *zap.Logger) (Page, error) {
	doc, err := goquery.NewDocumentFromReader(raw)
	if err != nil {
		logger.Error("newpage error", zap.Error(err))
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
		p.logger.Debug("get title return title")
		return p.doc.Find("title").First().Text()
	}
}

func (p *page) GetLinks(ctx context.Context) []string {
	select {
	case <-ctx.Done():
		p.logger.Debug("context done in getlinks")
		return nil
	default:
		var urls []string
		p.doc.Find("a").Each(func(_ int, s *goquery.Selection) {
			url, ok := s.Attr("href")
			p.logger.Debug("trying to get links")
			if ok {
				urls = append(urls, url)
				logMsg := fmt.Sprintf("write url %s in slise urls", url)
				p.logger.Debug(logMsg)
			}
		})
		return urls
	}
}

type Requester interface {
	Get(ctx context.Context, url string) (Page, error)
}

type request struct {
	timeout time.Duration
	logger  *zap.Logger
}

func NewRequest(timeout time.Duration, logger *zap.Logger) request {
	logger.Info("new request initialize")
	return request{timeout: timeout, logger: logger}
}

func (r request) Get(ctx context.Context, url string) (Page, error) {
	select {
	case <-ctx.Done():
		r.logger.Debug("context done in get")
		return nil, nil
	default:
		cl := &http.Client{
			Timeout: r.timeout,
		}
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			logMsg := fmt.Sprintf("error by get new request, url: %s", url)
			r.logger.Error(logMsg)
			return nil, err
		}
		body, err := cl.Do(req)
		if err != nil {
			r.logger.Error("http.Client error", zap.Error(err))
			return nil, err
		}
		defer body.Body.Close()
		page, err := NewPage(body.Body, r.logger)
		if err != nil {
			r.logger.Error("get new page error", zap.Error(err))
			return nil, err
		}
		return page, nil
	}
	return nil, nil
}

//Crawler - интерфейс (контракт) краулера
type Crawler interface {
	Scan(ctx context.Context, url string, depth uint64)
	ChanResult() <-chan CrawlResult
}

type crawler struct {
	r       Requester
	res     chan CrawlResult
	visited map[string]struct{}
	mu      sync.RWMutex
	config  *Config
	logger  *zap.Logger
}

func NewCrawler(r Requester, config *Config, logger *zap.Logger) *crawler {
	return &crawler{
		r:       r,
		res:     make(chan CrawlResult),
		visited: make(map[string]struct{}),
		mu:      sync.RWMutex{},
		config:  config,
		logger:  logger,
	}
}

func (c *crawler) Scan(ctx context.Context, url string, depth uint64) {
	if depth > c.config.MaxDepth { //Проверяем то, что есть запас по глубине
		logMsg := fmt.Sprintf("actual depth: %d, maxdepth: %d", depth, c.config.MaxDepth)
		c.logger.Debug(logMsg)
		return
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") { // Отсекаем невалидные ссылки
		logMsg := fmt.Sprintf("%s is not a valid link", url)
		c.logger.Warn(logMsg)
		return
	}
	c.mu.RLock()
	_, ok := c.visited[url] //Проверяем, что мы ещё не смотрели эту страницу
	c.mu.RUnlock()
	if ok {
		return
	}
	select {
	case <-ctx.Done(): //Если контекст завершен - прекращаем выполнение
		c.logger.Debug("crawler stopped by context")
		return
	default:
		page, err := c.r.Get(ctx, url) //Запрашиваем страницу через Requester
		if err != nil {
			c.res <- CrawlResult{Err: err} //Записываем ошибку в канал
			return
		}
		c.mu.Lock()
		c.visited[url] = struct{}{} //Помечаем страницу просмотренной
		c.mu.Unlock()

		c.res <- CrawlResult{ //Отправляем результаты в канал
			Title: page.GetTitle(ctx),
			Url:   url,
		}
		for _, link := range page.GetLinks(ctx) {
			go c.Scan(ctx, link, depth+1) //На все полученные ссылки запускаем новую рутину сборки
		}
	}
}

func (c *crawler) ChanResult() <-chan CrawlResult {
	return c.res
}

//Config - структура для конфигурации
type Config struct {
	MaxDepth   uint64
	MaxResults int
	MaxErrors  int
	Url        string
	AppTimeout int //in seconds
	ReqTimeout int //in seconds
}

func main() {

	cfg := Config{
		MaxDepth:   100,
		MaxResults: 1000,
		MaxErrors:  5000,
		Url:        "https://telegram.org",
		AppTimeout: 20,
		ReqTimeout: 2,
	}

	logger, err := zap.NewDevelopment()
	if err != nil {
		logger.Error("can't initialize logger", zap.Error(err))
	}
	defer logger.Sync()

	var cr Crawler
	var r Requester

	r = NewRequest(time.Duration(cfg.AppTimeout)*time.Second, logger)
	cr = NewCrawler(r, &cfg, logger)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.AppTimeout)*time.Second) // Общий таймаут
	crCtx, _ := context.WithTimeout(context.Background(), time.Duration(cfg.ReqTimeout)*time.Second)    // Таймаут парсера, получения ссылок, формирования заголовков

	go cr.Scan(crCtx, cfg.Url, 0) //Запускаем краулер в отдельной рутине

	go processResult(ctx, cancel, cr, cfg, logger) //Обрабатываем результаты в отдельной рутине

	sigIntCh := make(chan os.Signal)        //Создаем канал для приема сигналов SIGINT
	signal.Notify(sigIntCh, syscall.SIGINT) //Подписываемся на сигнал SIGINT

	sigUsr1Ch := make(chan os.Signal)         //Создаем канал для приема сигналов SIGUSR1
	signal.Notify(sigUsr1Ch, syscall.SIGUSR1) //Подписываемся на сигнал SIGUSR1
	for {
		select {
		case <-ctx.Done(): //Если всё завершили - выходим
			logger.Debug("context done")
			return
		case <-sigIntCh:
			cancel() //Если пришёл сигнал SigInt - завершаем контекст
			logger.Info("sigint detected. program shutdown")
		case <-sigUsr1Ch:
			atomic.AddUint64(&cfg.MaxDepth, 2) //Если пришел сигнал SigUsr1 - увеличиваем MaxDepth на 2
			logMsg := fmt.Sprintf("sigusr1 detected, new value of depth:%d", cfg.MaxDepth)
			logger.Info(logMsg)
		}
	}
}

func processResult(ctx context.Context, cancel func(), cr Crawler, cfg Config, logger *zap.Logger) {

	var maxResult, maxErrors = cfg.MaxResults, cfg.MaxErrors
	for {
		select {
		case <-ctx.Done():
			logger.Debug("context done")
			return
		case msg := <-cr.ChanResult():
			if msg.Err != nil {
				maxErrors--
				logger.Error(fmt.Sprintf("crawler result return err: %s", msg.Err.Error()),
					zap.Field{Key: "error", String: msg.Err.Error(), Type: zapcore.StringType})
				if maxErrors <= 0 {
					cancel()
					logger.Debug("errors limit is over. program shutdown")
					return
				}
			} else {
				maxResult--
				logMsg := fmt.Sprintf("crawler result: [url: %s] Title: %s\n", msg.Url, msg.Title)
				logger.Info(logMsg)
				if maxResult <= 0 {
					cancel()
					logger.Debug("maximum of results is over. program shutdown")
					return
				}
			}
		}
	}
}
