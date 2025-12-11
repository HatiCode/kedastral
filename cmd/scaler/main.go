package main

import (
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/HatiCode/kedastral/cmd/scaler/config"
	"github.com/HatiCode/kedastral/cmd/scaler/logger"
	"github.com/HatiCode/kedastral/cmd/scaler/metrics"
	"github.com/HatiCode/kedastral/cmd/scaler/router"
	pb "github.com/HatiCode/kedastral/pkg/api/externalscaler"
	"github.com/HatiCode/kedastral/pkg/httpx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

func main() {
	cfg := config.ParseFlags()
	log := logger.New(cfg)
	m := metrics.New()

	log.Info("starting kedastral scaler",
		"listen", cfg.Listen,
		"forecaster_url", cfg.ForecasterURL,
		"lead_time", cfg.LeadTime,
	)

	scaler := New(cfg.ForecasterURL, cfg.LeadTime, log, m)

	grpcServer := grpc.NewServer()

	pb.RegisterExternalScalerServer(grpcServer, scaler)

	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	reflection.Register(grpcServer)

	lis, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		log.Error("failed to listen", "error", err)
		os.Exit(1)
	}

	go func() {
		log.Info("grpc server listening", "address", cfg.Listen)
		if err := grpcServer.Serve(lis); err != nil {
			log.Error("grpc server failed", "error", err)
			os.Exit(1)
		}
	}()

	httpMux := router.SetupRoutes(log)
	httpServer := httpx.NewServer(":8082", httpMux, log)

	go func() {
		if err := httpServer.Start(); err != nil {
			log.Error("http server failed", "error", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	sig := <-sigChan
	log.Info("received shutdown signal", "signal", sig)

	log.Info("shutting down grpc server")
	grpcServer.GracefulStop()

	log.Info("shutting down http server")
	if err := httpServer.Stop(10 * time.Second); err != nil {
		log.Error("http server shutdown error", "error", err)
	}

	log.Info("shutdown complete")
}
