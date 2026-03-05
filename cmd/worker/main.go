package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"pharmalink/server/internal/asynqjobs"
	"pharmalink/server/internal/cache"
	"pharmalink/server/internal/config"
	"pharmalink/server/internal/db"
	"pharmalink/server/internal/logger"
	"pharmalink/server/internal/modules/outbox"
	"pharmalink/server/internal/worker"

	"github.com/hibiken/asynq"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	log := logger.New(cfg.AppEnv)

	dbConn, err := db.NewPostgres(cfg.Postgres, log)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect postgres")
	}
	sqlDB, _ := dbConn.DB()
	defer sqlDB.Close()

	redisClient := cache.NewRedis(cfg.Redis)
	if err := cache.Ping(context.Background(), redisClient); err != nil {
		log.Fatal().Err(err).Msg("failed to connect redis")
	}
	defer redisClient.Close()

	asynqClient := asynq.NewClient(asynq.RedisClientOpt{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer asynqClient.Close()

	outboxSvc := outbox.NewService(dbConn, cfg.OutboxChannel)
	enqueuer := asynqjobs.NewEnqueuer(asynqClient)
	processor := worker.NewOutboxProcessor(outboxSvc, redisClient, nil, enqueuer, log)
	listener := worker.NewOutboxListener(cfg.Postgres, cfg.OutboxChannel, processor, log)
	asynqRunner := worker.NewAsynqRunner(cfg.Redis, processor, log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := listener.Run(ctx); err != nil {
			log.Fatal().Err(err).Msg("outbox listener failed")
		}
	}()

	go func() {
		if err := asynqRunner.Run(ctx, cfg.WorkerConcurrency); err != nil {
			log.Fatal().Err(err).Msg("asynq server failed")
		}
	}()

	log.Info().Msg("worker started")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	cancel()
	time.Sleep(2 * time.Second)
}

