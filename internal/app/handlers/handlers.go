package handlers

import (
	"context"
	"crawler/config"
	"crawler/internal/usecase"
	"fmt"

	"go.uber.org/zap"
)

func ProcessResult(ctx context.Context, cancel func(), cr usecase.Crawler, cfg *config.Config, logger *zap.Logger) {
	var maxResult, maxErrors = cfg.MaxResults, cfg.MaxErrors
	for {
		select {
		case <-ctx.Done():
			logger.Debug("context done in process result")
			return
		case msg := <-cr.ChanResult():
			if msg.Err != nil {
				maxErrors--
				logMsg := fmt.Sprintf("crawler result return err: %s\n", msg.Err.Error())
				logger.Error(logMsg)
				if maxErrors <= 0 {
					cancel()
					logger.Debug("errors limit is over. programm shutdown")
					return
				}
			} else {
				maxResult--
				logMsg := fmt.Sprintf("crawler result: [url: %s] Title: %s\n", msg.Url, msg.Title)
				logger.Info(logMsg)
				if maxResult <= 0 {
					cancel()
					logger.Debug("Maximum of results is over. Programm shutdown")
					return
				}
			}
		}
	}
}
