package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all environment-driven configuration for Notifyx.
// Values are read at startup; the app fails fast if required values are missing.
type Config struct {
	// Server
	Port string
	Env  string // "development" | "production"

	// PostgreSQL
	DatabaseURL string

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
}

// Load reads .env (if present) then overlays OS environment variables.
// Call once at startup; pass the returned Config everywhere via dependency injection.
func Load() (*Config, error) {
	// godotenv.Load is a no-op if .env does not exist — safe to call always.
	_ = godotenv.Load()

	cfg := &Config{
		Port:                    getEnv("PORT", "8080"),
		Env:                     getEnv("APP_ENV", "development"),
		DatabaseURL:             getEnv("DATABASE_URL", ""),
		RedisURL:                getEnv("REDIS_URL", ""),
		KafkaBootstrapServers:   getEnv("KAFKA_BOOTSTRAP_SERVERS", ""),
		KafkaAPIKey:             getEnv("KAFKA_API_KEY", ""),
		KafkaAPISecret:          getEnv("KAFKA_API_SECRET", ""),
		ResendAPIKey:            getEnv("RESEND_API_KEY", ""),
		FirebaseCredentialsJSON: getEnv("FIREBASE_CREDENTIALS_JSON", ""),
		Fast2SMSAPIKey:          getEnv("FAST2SMS_API_KEY", ""),
		DefaultRateLimitPerMin:  getEnvInt("DEFAULT_RATE_LIMIT_PER_MIN", 60),
		DefaultGlobalCap:        getEnvInt("DEFAULT_GLOBAL_CAP", 300),
	}

	return cfg, nil
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
