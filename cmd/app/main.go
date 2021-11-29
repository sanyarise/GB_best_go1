package main

import (
	"context"
	"crawler/config"
	"crawler/internal/app/crawler"
	"crawler/internal/app/handlers"
	"crawler/internal/app/requester"
	"crawler/internal/usecase"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"

	"go.uber.org/zap"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config-path", "config/config.toml", "path to config file in .toml format")

	logger, err := zap.NewDevelopment()
	if err != nil {
		logger.Error("can't initialize logger", zap.Error(err))
	}
	defer logger.Sync()

	flag.Parse()
	cfg := config.NewConfig()
	toml.DecodeFile(configPath, cfg)
	if err != nil {
		logger.Debug("can't find configs file. using defaul values: ", zap.Error(err))
	}

	var r usecase.Requester = requester.NewRequester(time.Duration(cfg.AppTimeout)*time.Second, logger, nil)
	var cr usecase.Crawler = crawler.NewCrawler(r, logger, cfg.MaxDepth)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.AppTimeout)*time.Second) // Общий таймаут
	crCtx, _ := context.WithTimeout(context.Background(), time.Duration(cfg.ReqTimeout)*time.Second)    // Таймаут парсера, получения ссылок, формирования заголовков

	go cr.Scan(crCtx, cfg.Url, 0)                           //Запускаем краулер в отдельной рутине
	go handlers.ProcessResult(ctx, cancel, cr, cfg, logger) //Обрабатываем результаты в отдельной рутине

	sigIntCh := make(chan os.Signal, 1)     //Создаем канал для приема сигналов SIGINT
	signal.Notify(sigIntCh, syscall.SIGINT) //Подписываемся на сигнал SIGINT

	sigUsr1Ch := make(chan os.Signal, 1)      //Создаем канал для приема сигналов SIGUSR1
	signal.Notify(sigUsr1Ch, syscall.SIGUSR1) //Подписываемся на сигнал SIGUSR1
	for {
		select {
		case <-ctx.Done(): //Если всё завершили - выходим
			logger.Debug("context done in main")
			return
		case <-sigIntCh:
			cancel() //Если пришёл сигнал SigInt - завершаем контекст
			logger.Info("sigint detected. program shutdown")
		case <-sigUsr1Ch:
			cr.IncMaxDepth(2) //Если пришел сигнал SigUsr1 - увеличиваем MaxDepth на 2
			logMsg := fmt.Sprintf("sigusr1 detected, new value of depth: %d", cfg.MaxDepth)
			logger.Info(logMsg)
		}
	}
}
