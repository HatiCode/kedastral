package models

import (
	"log/slog"
	"os"

	"github.com/HatiCode/kedastral/cmd/forecaster/config"
	"github.com/HatiCode/kedastral/pkg/models"
)

func New(cfg *config.Config, logger *slog.Logger) models.Model {
	stepSec := int(cfg.Step.Seconds())
	horizonSec := int(cfg.Horizon.Seconds())

	switch cfg.Model {
	case "arima":
		logger.Info("initializing ARIMA model",
			"p", cfg.ARIMA_P,
			"d", cfg.ARIMA_D,
			"q", cfg.ARIMA_Q,
		)
		return models.NewARIMAModel(cfg.Metric, stepSec, horizonSec, cfg.ARIMA_P, cfg.ARIMA_D, cfg.ARIMA_Q)

	case "baseline":
		logger.Info("initializing baseline model")
		return models.NewBaselineModel(cfg.Metric, stepSec, horizonSec)

	default:
		logger.Error("invalid model type", "model", cfg.Model)
		os.Exit(1)
	}

	return nil
}
