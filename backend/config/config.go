package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all environment-driven configuration for Notifyx.
// Values are read at startup; the app fails fast if required values are missing.
type Config struct {
	// Server
	Port     string
	Env      string // "development" | "production"
	LogLevel string // optional: debug|info|warn|error — overrides Env's default verbosity

	// PostgreSQL
	DatabaseURL string
	DBMaxConns  int // explicit cap — free-tier Postgres plans cap total connections low
	DBMinConns  int

	// Redis (Upstash)
	RedisURL string

	// Kafka (Confluent)
	KafkaBootstrapServers string
	KafkaAPIKey           string
	KafkaAPISecret        string

	// Resend (Email)
	ResendAPIKey string

	// Firebase FCM (Push)
	FirebaseCredentialsJSON string // path to service account JSON file

	// Fast2SMS (SMS)
	Fast2SMSAPIKey string

	// Rate limit defaults (used when tenant has no custom config)
	DefaultRateLimitPerMin int
	DefaultGlobalCap       int

	// Dedup
	DedupTTL time.Duration // idempotency-key reservation window
}

// Load reads .env (if present) then overlays OS environment variables.
// Call once at startup; pass the returned Config everywhere via dependency injection.
func Load() (*Config, error) {
	// godotenv.Load is a no-op if .env does not exist — safe to call always.
	_ = godotenv.Load()

	cfg := &Config{
		Port:                    getEnv("PORT", "8080"),
		Env:                     getEnv("APP_ENV", "development"),
		LogLevel:                getEnv("LOG_LEVEL", ""),
		DatabaseURL:             getEnv("DATABASE_URL", ""),
		DBMaxConns:              getEnvInt("DB_MAX_CONNS", 10),
		DBMinConns:              getEnvInt("DB_MIN_CONNS", 0),
		RedisURL:                getEnv("REDIS_URL", ""),
		KafkaBootstrapServers:   getEnv("KAFKA_BOOTSTRAP_SERVERS", ""),
		KafkaAPIKey:             getEnv("KAFKA_API_KEY", ""),
		KafkaAPISecret:          getEnv("KAFKA_API_SECRET", ""),
		ResendAPIKey:            getEnv("RESEND_API_KEY", ""),
		FirebaseCredentialsJSON: getEnv("FIREBASE_CREDENTIALS_JSON", ""),
		Fast2SMSAPIKey:          getEnv("FAST2SMS_API_KEY", ""),
		DefaultRateLimitPerMin:  getEnvInt("DEFAULT_RATE_LIMIT_PER_MIN", 60),
		DefaultGlobalCap:        getEnvInt("DEFAULT_GLOBAL_CAP", 300),
		DedupTTL:                time.Duration(getEnvInt("DEDUP_TTL_HOURS", 24)) * time.Hour,
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	return cfg, nil
}

// Validate fails fast on missing or nonsensical config instead of letting the app start
// and crash later with a more confusing error deep inside pool init or migrations.
func (c *Config) Validate() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if c.Env != "development" && c.Env != "production" {
		return fmt.Errorf(`APP_ENV must be "development" or "production", got %q`, c.Env)
	}
	if c.DBMaxConns < 1 {
		return fmt.Errorf("DB_MAX_CONNS must be at least 1, got %d", c.DBMaxConns)
	}
	if c.DBMinConns < 0 || c.DBMinConns > c.DBMaxConns {
		return fmt.Errorf("DB_MIN_CONNS must be between 0 and DB_MAX_CONNS (%d), got %d", c.DBMaxConns, c.DBMinConns)
	}
	if c.DedupTTL <= 0 {
		return fmt.Errorf("DEDUP_TTL_HOURS must be positive, got %v", c.DedupTTL)
	}
	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
