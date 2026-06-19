package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/rohit-bagade/notifyx/config"
	"github.com/rohit-bagade/notifyx/internal/api/handlers"
	"github.com/rohit-bagade/notifyx/internal/api/routes"
	"github.com/rohit-bagade/notifyx/internal/kafka/consumers"
	kafkaproducer "github.com/rohit-bagade/notifyx/internal/kafka/producer"
	"github.com/rohit-bagade/notifyx/internal/repository/postgres"
	"github.com/rohit-bagade/notifyx/pkg/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		println("failed to load config:", err.Error())
		os.Exit(1)
	}

	log, err := logger.New(cfg.Env)
	if err != nil {
		println("failed to init logger:", err.Error())
		os.Exit(1)
	}
	defer log.Sync()

	log.Infow("starting Notifyx", "env", cfg.Env, "port", cfg.Port)

	if err := postgres.RunMigrations(cfg.DatabaseURL, "migrations", log); err != nil {
		log.Fatal("migration failed", "error", err)
	}

	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL, log)
	if err != nil {
		log.Fatal("database connection failed", "error", err)
	}
	defer pool.Close()

	// Repositories.
	tenantRepo := postgres.NewTenantRepository(pool)
	apiKeyRepo := postgres.NewAPIKeyRepository(pool)
	channelRepo := postgres.NewTenantChannelRepository(pool)
	rateLimitRepo := postgres.NewTenantRateLimitRepository(pool)
	dlqRepo := postgres.NewDLQRepository(pool)

	// Kafka — only started when KAFKA_BOOTSTRAP_SERVERS is set.
	// This lets the app start without Kafka during local development / DB-only testing.
	var prod *kafkaproducer.Producer
	consumerCtx, cancelConsumers := context.WithCancel(context.Background())
	var consumerWg sync.WaitGroup

	if cfg.KafkaBootstrapServers != "" {
		prod, err = kafkaproducer.New(cfg.KafkaBootstrapServers, cfg.KafkaAPIKey, cfg.KafkaAPISecret, log)
		if err != nil {
			log.Fatal("create kafka producer failed", "error", err)
		}

		kafkaCfg := consumers.Config{
			BootstrapServers: cfg.KafkaBootstrapServers,
			APIKey:           cfg.KafkaAPIKey,
			APISecret:        cfg.KafkaAPISecret,
		}

		startConsumers(consumerCtx, &consumerWg, kafkaCfg, prod, dlqRepo, log)
		log.Infow("kafka consumers started", "brokers", cfg.KafkaBootstrapServers)
	} else {
		log.Warnw("KAFKA_BOOTSTRAP_SERVERS not set — Kafka producer and consumers disabled")
	}

	// HTTP handlers and routes.
	h := &routes.Handlers{
		Health: handlers.NewHealthHandler(),
		Tenant: handlers.NewTenantHandler(&handlers.TenantService{
			Tenants:    tenantRepo,
			APIKeys:    apiKeyRepo,
			Channels:   channelRepo,
			RateLimits: rateLimitRepo,
		}, log),
	}
	e := routes.Setup(h, apiKeyRepo, log)

	// Start HTTP server in its own goroutine so the signal handler below can run.
	go func() {
		if err := e.Start(":" + cfg.Port); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error", "error", err)
		}
	}()

	log.Infow("Notifyx ready", "port", cfg.Port)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Infow("shutdown signal received, draining...")

	// 1. Stop consumers — let in-flight messages complete.
	cancelConsumers()
	consumerWg.Wait()

	// 2. Flush the producer — ensure no messages are lost in the send buffer.
	if prod != nil {
		prod.Close()
	}

	// 3. Shut down the HTTP server — give in-flight requests 10 s to finish.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(shutdownCtx); err != nil {
		log.Errorw("server shutdown error", "error", err)
	}

	log.Infow("Notifyx stopped")
}

// startConsumers creates all five Kafka consumers and runs each in its own goroutine.
// consumerWg is Done'd when each goroutine exits, so main can wait for a clean drain.
func startConsumers(
	ctx context.Context,
	wg *sync.WaitGroup,
	cfg consumers.Config,
	prod *kafkaproducer.Producer,
	dlqRepo *postgres.DLQRepository,
	log *logger.Logger,
) {
	type entry struct {
		name string
		c    *consumers.Consumer
	}

	constructors := []struct {
		name string
		fn   func() (*consumers.Consumer, error)
	}{
		{"email", func() (*consumers.Consumer, error) { return consumers.NewEmailConsumer(cfg, prod, log) }},
		{"push", func() (*consumers.Consumer, error) { return consumers.NewPushConsumer(cfg, prod, log) }},
		{"sms", func() (*consumers.Consumer, error) { return consumers.NewSMSConsumer(cfg, prod, log) }},
		{"inapp", func() (*consumers.Consumer, error) { return consumers.NewInAppConsumer(cfg, prod, log) }},
		{"dlq", func() (*consumers.Consumer, error) { return consumers.NewDLQConsumer(cfg, dlqRepo, log) }},
	}

	for _, ctor := range constructors {
		c, err := ctor.fn()
		if err != nil {
			log.Fatal("create consumer failed", "consumer", ctor.name, "error", err)
		}
		wg.Add(1)
		go func(name string, consumer *consumers.Consumer) {
			defer wg.Done()
			consumer.Run(ctx)
		}(ctor.name, c)
	}
}
