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
	"github.com/rohit-bagade/notifyx/internal/channels/email"
	"github.com/rohit-bagade/notifyx/internal/channels/push"
	"github.com/rohit-bagade/notifyx/internal/channels/sms"
	"github.com/rohit-bagade/notifyx/internal/dedup"
	"github.com/rohit-bagade/notifyx/internal/kafka/consumers"
	kafkaproducer "github.com/rohit-bagade/notifyx/internal/kafka/producer"
	"github.com/rohit-bagade/notifyx/internal/offlinequeue"
	"github.com/rohit-bagade/notifyx/internal/presence"
	"github.com/rohit-bagade/notifyx/internal/ratelimit"
	"github.com/rohit-bagade/notifyx/internal/repository/postgres"
	redisrepo "github.com/rohit-bagade/notifyx/internal/repository/redis"
	"github.com/rohit-bagade/notifyx/internal/ws"
	"github.com/rohit-bagade/notifyx/pkg/logger"
	goredis "github.com/redis/go-redis/v9"
)

// replayInterval is how often the rate-limit replayer re-checks queued_rate_limited
// notifications to see if their tenant's window has cleared.
const replayInterval = 15 * time.Second

func main() {
	cfg, err := config.Load()
	if err != nil {
		println("failed to load config:", err.Error())
		os.Exit(1)
	}

	log, err := logger.New(cfg.Env, cfg.LogLevel)
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
	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL, int32(cfg.DBMaxConns), int32(cfg.DBMinConns), log)
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
	notificationRepo := postgres.NewNotificationRepository(pool)
	deliveryRepo := postgres.NewNotificationDeliveryRepository(pool)

	bgCtx, cancelBg := context.WithCancel(context.Background())
	var bgWg sync.WaitGroup

	// Redis — rate limiting, dedup, and in-app presence/offline-queue, only started when
	// REDIS_URL is set. Like Kafka, this lets the app run locally without Redis; sends then
	// skip rate limiting/dedup, and in-app delivery falls back to the Kafka retry/DLQ path
	// for any recipient who's offline at delivery time.
	var limiter *ratelimit.Limiter
	var deduplicator *dedup.Deduplicator
	var presenceTracker *presence.Tracker
	var offlineQueue *offlinequeue.Queue
	var redisClient *goredis.Client

	if cfg.RedisURL != "" {
		redisClient, err = redisrepo.NewClient(ctx, cfg.RedisURL, log)
		if err != nil {
			log.Fatal("create redis client failed", "error", err)
		}
		defer redisClient.Close()

		limiter = ratelimit.New(redisClient, tenantRepo, rateLimitRepo, cfg.DefaultRateLimitPerMin)
		deduplicator = dedup.New(redisClient, cfg.DedupTTL)
		presenceTracker = presence.New(redisClient)
		offlineQueue = offlinequeue.New(redisClient)

		log.Infow("rate limiting, dedup, and in-app presence/offline-queue enabled")
	} else {
		log.Warnw("REDIS_URL not set — rate limiting, dedup, and in-app offline queue disabled")
	}

	// In-app WebSocket hub — always created (no external dependency), so direct in-app
	// delivery to currently-connected clients works even without Redis.
	hub := ws.NewHub(log)

	// Kafka — only started when KAFKA_BOOTSTRAP_SERVERS is set.
	// This lets the app start without Kafka during local development / DB-only testing.
	var prod *kafkaproducer.Producer

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

		startConsumers(bgCtx, &bgWg, kafkaCfg, cfg, prod, hub, offlineQueue, dlqRepo, notificationRepo, deliveryRepo, log)
		log.Infow("kafka consumers started", "brokers", cfg.KafkaBootstrapServers)
	} else {
		log.Warnw("KAFKA_BOOTSTRAP_SERVERS not set — Kafka producer and consumers disabled")
	}

	// Rate-limit replayer — needs the real (possibly-nil) Kafka producer, so it's built
	// after the Kafka block runs rather than alongside the rest of Redis setup above.
	if redisClient != nil {
		replayer := ratelimit.NewReplayer(notificationRepo, deliveryRepo, limiter, prod, log)
		bgWg.Add(1)
		go func() {
			defer bgWg.Done()
			replayer.Run(bgCtx, replayInterval)
		}()
	}

	// HTTP handlers and routes.
	h := &routes.Handlers{
		Health: handlers.NewHealthHandler(),
		WS:     ws.NewHandler(hub, presenceTracker, offlineQueue, apiKeyRepo, log),
		Tenant: handlers.NewTenantHandler(&handlers.TenantService{
			Tenants:              tenantRepo,
			APIKeys:              apiKeyRepo,
			Channels:             channelRepo,
			RateLimits:           rateLimitRepo,
			DefaultGlobalRateCap: cfg.DefaultGlobalCap,
			Pool:                 pool,
		}, log),
		Notification: handlers.NewNotificationHandler(&handlers.NotificationService{
			Notifications: notificationRepo,
			Deliveries:    deliveryRepo,
			Channels:      channelRepo,
			Producer:      prod,
			RateLimiter:   limiter,
			Dedup:         deduplicator,
			Pool:          pool,
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

	// 1. Stop background workers (Kafka consumers, rate-limit replayer) — let in-flight
	// work complete.
	cancelBg()
	bgWg.Wait()

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

// startConsumers creates the Kafka consumers and runs each in its own goroutine.
// bgWg is Done'd when each goroutine exits, so main can wait for a clean drain.
//
// The email/push/sms consumers are only started when their provider credentials are
// configured — mirrors the existing Kafka/Redis-optional convention, so the app still
// starts locally without every provider's API key set. inapp and dlq have no external
// provider dependency and always start.
func startConsumers(
	ctx context.Context,
	wg *sync.WaitGroup,
	kafkaCfg consumers.Config,
	cfg *config.Config,
	prod *kafkaproducer.Producer,
	hub *ws.Hub,
	offlineQueue *offlinequeue.Queue,
	dlqRepo *postgres.DLQRepository,
	notificationRepo *postgres.NotificationRepository,
	deliveryRepo *postgres.NotificationDeliveryRepository,
	log *logger.Logger,
) {
	constructors := []struct {
		name string
		fn   func() (*consumers.Consumer, error)
	}{
		{"inapp", func() (*consumers.Consumer, error) {
			return consumers.NewInAppConsumer(kafkaCfg, prod, hub, offlineQueue, deliveryRepo, log)
		}},
		{"dlq", func() (*consumers.Consumer, error) {
			return consumers.NewDLQConsumer(kafkaCfg, dlqRepo, notificationRepo, deliveryRepo, log)
		}},
	}

	if cfg.ResendAPIKey != "" {
		emailClient := email.NewClient(cfg.ResendAPIKey, cfg.ResendFromEmail)
		constructors = append(constructors, struct {
			name string
			fn   func() (*consumers.Consumer, error)
		}{"email", func() (*consumers.Consumer, error) {
			return consumers.NewEmailConsumer(kafkaCfg, prod, emailClient, deliveryRepo, log)
		}})
	} else {
		log.Warnw("RESEND_API_KEY not set — email consumer disabled")
	}

	if cfg.Fast2SMSAPIKey != "" {
		smsClient := sms.NewClient(cfg.Fast2SMSAPIKey)
		constructors = append(constructors, struct {
			name string
			fn   func() (*consumers.Consumer, error)
		}{"sms", func() (*consumers.Consumer, error) {
			return consumers.NewSMSConsumer(kafkaCfg, prod, smsClient, deliveryRepo, log)
		}})
	} else {
		log.Warnw("FAST2SMS_API_KEY not set — sms consumer disabled")
	}

	if cfg.FirebaseCredentialsJSON != "" {
		pushClient, err := push.NewClient(cfg.FirebaseCredentialsJSON)
		if err != nil {
			log.Fatalw("create fcm client failed", "error", err)
		}
		constructors = append(constructors, struct {
			name string
			fn   func() (*consumers.Consumer, error)
		}{"push", func() (*consumers.Consumer, error) {
			return consumers.NewPushConsumer(kafkaCfg, prod, pushClient, deliveryRepo, log)
		}})
	} else {
		log.Warnw("FIREBASE_CREDENTIALS_JSON not set — push consumer disabled")
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
