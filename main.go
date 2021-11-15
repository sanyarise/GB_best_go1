package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

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
	doc *goquery.Document
}

func NewPage(raw io.Reader) (Page, error) {
	doc, err := goquery.NewDocumentFromReader(raw)
	if err != nil {
		return nil, err
	}
	return &page{doc: doc}, nil
}

func (p *page) GetTitle(ctx context.Context) string {
	select {
	case <-ctx.Done():
		log.Println("Timeout of GetTitle is over")
		return ""
	default:
		return p.doc.Find("title").First().Text()
	}
}

func (p *page) GetLinks(ctx context.Context) []string {
	select {
	case <-ctx.Done():
		log.Println("Timeout of GetLinks is over")
		return nil
	default:
		var urls []string
		p.doc.Find("a").Each(func(_ int, s *goquery.Selection) {
			url, ok := s.Attr("href")
			if ok {
				urls = append(urls, url)
			}
		})
		return urls
	}
}

type Requester interface {
	Get(ctx context.Context, url string) (Page, error)
}

type requester struct {
	timeout time.Duration
}

func NewRequester(timeout time.Duration) requester {
	return requester{timeout: timeout}
}

func (r requester) Get(ctx context.Context, url string) (Page, error) {
	select {
	case <-ctx.Done():
		return nil, nil
	default:
		cl := &http.Client{
			Timeout: r.timeout,
		}
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		body, err := cl.Do(req)
		if err != nil {
			return nil, err
		}
		defer body.Body.Close()
		page, err := NewPage(body.Body)
		if err != nil {
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
}

func NewCrawler(r Requester, config *Config) *crawler {
	return &crawler{
		r:       r,
		res:     make(chan CrawlResult),
		visited: make(map[string]struct{}),
		mu:      sync.RWMutex{},
		config:  config,
	}
}

func (c *crawler) Scan(ctx context.Context, url string, depth uint64) {
	if depth > c.config.MaxDepth { //Проверяем то, что есть запас по глубине
		return
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") { // Отсекаем невалидные ссылки
		log.Printf("%s is not a valid link", url)
		return
	}
	c.mu.RLock()
	_, ok := c.visited[url] //Проверяем, что мы ещё не смотрели эту страницу
	c.mu.RUnlock()
	if ok {
		return
	}
	log.Printf("URL %s was checked", url)
	select {
	case <-ctx.Done(): //Если контекст завершен - прекращаем выполнение
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
	var cr Crawler
	var r Requester

	r = NewRequester(time.Duration(cfg.AppTimeout) * time.Second)
	cr = NewCrawler(r, &cfg)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.AppTimeout)*time.Second) // Общий таймаут
	crCtx, _ := context.WithTimeout(context.Background(), time.Duration(cfg.ReqTimeout)*time.Second)    // Таймаут парсера, получения ссылок, формирования заголовков
	
	go cr.Scan(crCtx, cfg.Url, 0)                                                                       //Запускаем краулер в отдельной рутине
	
	go processResult(ctx, cancel, cr, cfg)                                                              //Обрабатываем результаты в отдельной рутине

	sigIntCh := make(chan os.Signal)        //Создаем канал для приема сигналов SIGINT
	signal.Notify(sigIntCh, syscall.SIGINT) //Подписываемся на сигнал SIGINT

	sigUsr1Ch := make(chan os.Signal)         //Создаем канал для приема сигналов SIGUSR1
	signal.Notify(sigUsr1Ch, syscall.SIGUSR1) //Подписываемся на сигнал SIGUSR1
	for {
		select {
		case <-ctx.Done(): //Если всё завершили - выходим
			log.Println("Timeout in main")
			return
		case <-sigIntCh:
			cancel() //Если пришёл сигнал SigInt - завершаем контекст
			log.Println("sigint detected. program shutdown")
		case <-sigUsr1Ch:
			atomic.AddUint64(&cfg.MaxDepth, 2) //Если пришел сигнал SigUsr1 - увеличиваем MaxDepth на 2
			log.Printf("sigusr1 detected, new value of depth:%d", cfg.MaxDepth)
		}
	}
}

func processResult(ctx context.Context, cancel func(), cr Crawler, cfg Config) {
	var maxResult, maxErrors = cfg.MaxResults, cfg.MaxErrors
	for {
		select {
		case <-ctx.Done():
			log.Println("Timeout process result")
			return
		case msg := <-cr.ChanResult():
			if msg.Err != nil {
				maxErrors--
				log.Printf("crawler result return err: %s\n", msg.Err.Error())
				if maxErrors <= 0 {
					cancel()
					log.Println("Errors limit is over. Programm shutdown")
					return
				}
			} else {
				maxResult--
				log.Printf("crawler result: [url: %s] Title: %s\n", msg.Url, msg.Title)
				if maxResult <= 0 {
					cancel()
					log.Println("Maximum of results is over. Programm shutdown")
					return
				}
			}
		}
	}
}
