package main

import (
	"log/slog"
	"os"

	"github.com/common-nighthawk/go-figure"
	"github.com/hibiken/asynq"
	sharedconfig "github.com/mrhumster/go-shared/config"
	sharedmetrics "github.com/mrhumster/go-shared/metrics"
	sharedworker "github.com/mrhumster/go-shared/worker"
	"github.com/mrhumster/mailer-service/internal/mailer"
	"github.com/mrhumster/mailer-service/internal/queue"
)

var (
	version   = "dev"
	buildDate = "unknown"
)

func main() {
	figure.NewFigure("mailer "+version, "graffiti", true).Print()

	opts := &slog.HandlerOptions{
		Level:     slog.LevelDebug,
		AddSource: true,
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, opts)))

	cfg, err := sharedconfig.LoadConfig()
	if err != nil {
		slog.Error("error load config")
		os.Exit(1)
	}

	var sender mailer.EmailSender
	if cfg.Mail.SenderAddr != "" {
		sender = mailer.NewSMTPSender(mailer.SMTPConfig{
			Addr: cfg.Mail.SenderAddr,
			User: cfg.Mail.SenderUser,
			Pass: cfg.Mail.SenderPass,
			From: cfg.Mail.From,
		})
		slog.Info("mailer mode: SMTP", "addr", cfg.Mail.SenderAddr, "from", cfg.Mail.From)
	} else {
		sender = mailer.LogSender{}
		slog.Info("mailer mode: LOG (SMTP_ADDR empty)")
	}

	srv, err := sharedworker.NewAsynqServer(sharedworker.Options{
		Addr:            cfg.Redis.Addr,
		Password:        cfg.Redis.Password,
		DB:              cfg.Redis.DB,
		Concurrency:     cfg.Worker.Concurrency,
		ShutdownTimeout: cfg.Worker.ShutdownTimeout,
		Queues:          map[string]int{queue.TaskEmailQueue: 6},
		MetricsAddr:     cfg.Server.MetricsAddr,
	})
	if err != nil {
		slog.Error("error init asynq worker", "error", err)
		os.Exit(1)
	}

	handler := queue.NewHandleEmailVerification(sender, cfg.Mail.FrontendURL)
	mux := asynq.NewServeMux()
	mux.HandleFunc(
		queue.TaskEmailVerification,
		sharedmetrics.Instrument(queue.TaskEmailVerification, handler.HandleEmailVerificationTask),
	)

	slog.Info("mailer worker started...")
	if err := srv.Run(mux); err != nil {
		slog.Error("could not run asynq server:", "error", err.Error())
		os.Exit(1)
	}
}