package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setValidEnv sets the minimum env vars required for a valid config and
// returns nothing — callers override individual vars per test case.
func setValidEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db")
	t.Setenv("APP_ENV", "development")
}

func TestLoad_ValidConfig_Defaults(t *testing.T) {
	setValidEnv(t)

	cfg, err := Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "8080", cfg.Port)
	assert.Equal(t, "development", cfg.Env)
	assert.Equal(t, "", cfg.LogLevel)
	assert.Equal(t, "postgres://user:pass@localhost:5432/db", cfg.DatabaseURL)
	assert.Equal(t, 10, cfg.DBMaxConns)
	assert.Equal(t, 0, cfg.DBMinConns)
	assert.Equal(t, 60, cfg.DefaultRateLimitPerMin)
	assert.Equal(t, 300, cfg.DefaultGlobalCap)
	assert.Equal(t, 24*time.Hour, cfg.DedupTTL)
}

func TestLoad_ValidConfig_CustomValues(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/db")
	t.Setenv("APP_ENV", "production")
	t.Setenv("PORT", "9090")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("DB_MAX_CONNS", "20")
	t.Setenv("DB_MIN_CONNS", "5")
	t.Setenv("DEFAULT_RATE_LIMIT_PER_MIN", "100")
	t.Setenv("DEFAULT_GLOBAL_CAP", "500")
	t.Setenv("DEDUP_TTL_HOURS", "48")
	t.Setenv("RESEND_API_KEY", "key123")
	t.Setenv("RESEND_FROM_EMAIL", "noreply@example.com")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "9090", cfg.Port)
	assert.Equal(t, "production", cfg.Env)
	assert.Equal(t, "debug", cfg.LogLevel)
	assert.Equal(t, 20, cfg.DBMaxConns)
	assert.Equal(t, 5, cfg.DBMinConns)
	assert.Equal(t, 100, cfg.DefaultRateLimitPerMin)
	assert.Equal(t, 500, cfg.DefaultGlobalCap)
	assert.Equal(t, 48*time.Hour, cfg.DedupTTL)
	assert.Equal(t, "key123", cfg.ResendAPIKey)
	assert.Equal(t, "noreply@example.com", cfg.ResendFromEmail)
}

func TestLoad_MissingDatabaseURL(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	// DATABASE_URL intentionally not set.

	cfg, err := Load()
	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "DATABASE_URL is required")
}

func TestLoad_InvalidAppEnv(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/db")
	t.Setenv("APP_ENV", "staging")

	cfg, err := Load()
	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), `APP_ENV must be "development" or "production"`)
}

func TestLoad_InvalidIntField_FallsBackToDefault(t *testing.T) {
	// getEnvInt silently falls back to the default when the value doesn't parse —
	// this test documents that behavior rather than asserting an error.
	setValidEnv(t)
	t.Setenv("DB_MAX_CONNS", "not-a-number")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 10, cfg.DBMaxConns) // fallback default
}

func TestLoad_DBMaxConnsLessThanOne(t *testing.T) {
	setValidEnv(t)
	t.Setenv("DB_MAX_CONNS", "0")

	cfg, err := Load()
	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "DB_MAX_CONNS must be at least 1")
}

func TestLoad_DBMinConnsNegative(t *testing.T) {
	setValidEnv(t)
	t.Setenv("DB_MIN_CONNS", "-1")

	cfg, err := Load()
	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "DB_MIN_CONNS must be between 0 and DB_MAX_CONNS")
}

func TestLoad_DBMinConnsGreaterThanMax(t *testing.T) {
	setValidEnv(t)
	t.Setenv("DB_MAX_CONNS", "5")
	t.Setenv("DB_MIN_CONNS", "10")

	cfg, err := Load()
	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "DB_MIN_CONNS must be between 0 and DB_MAX_CONNS (5), got 10")
}

func TestLoad_DedupTTLNonPositive(t *testing.T) {
	setValidEnv(t)
	t.Setenv("DEDUP_TTL_HOURS", "0")

	cfg, err := Load()
	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "DEDUP_TTL_HOURS must be positive")
}

func TestLoad_DedupTTLNegative(t *testing.T) {
	setValidEnv(t)
	t.Setenv("DEDUP_TTL_HOURS", "-5")

	cfg, err := Load()
	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "DEDUP_TTL_HOURS must be positive")
}

func TestLoad_ResendAPIKeyWithoutFromEmail(t *testing.T) {
	setValidEnv(t)
	t.Setenv("RESEND_API_KEY", "some-key")
	// RESEND_FROM_EMAIL intentionally not set.

	cfg, err := Load()
	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "RESEND_FROM_EMAIL is required when RESEND_API_KEY is set")
}

func TestLoad_ResendAPIKeyWithFromEmail_OK(t *testing.T) {
	setValidEnv(t)
	t.Setenv("RESEND_API_KEY", "some-key")
	t.Setenv("RESEND_FROM_EMAIL", "noreply@example.com")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "some-key", cfg.ResendAPIKey)
	assert.Equal(t, "noreply@example.com", cfg.ResendFromEmail)
}

func TestLoad_NoResendAPIKey_FromEmailNotRequired(t *testing.T) {
	setValidEnv(t)
	// Neither RESEND_API_KEY nor RESEND_FROM_EMAIL set — should be valid.

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "", cfg.ResendAPIKey)
	assert.Equal(t, "", cfg.ResendFromEmail)
}

func TestValidate_DirectCall(t *testing.T) {
	cfg := &Config{
		DatabaseURL: "postgres://localhost/db",
		Env:         "production",
		DBMaxConns:  10,
		DBMinConns:  2,
		DedupTTL:    time.Hour,
	}
	assert.NoError(t, cfg.Validate())
}

func TestGetEnv_FallbackWhenUnset(t *testing.T) {
	t.Setenv("SOME_UNSET_VAR_FOR_TEST", "")
	assert.Equal(t, "fallback", getEnv("SOME_UNSET_VAR_FOR_TEST_NOT_SET", "fallback"))
}

func TestGetEnv_UsesSetValue(t *testing.T) {
	t.Setenv("SOME_SET_VAR_FOR_TEST", "actual")
	assert.Equal(t, "actual", getEnv("SOME_SET_VAR_FOR_TEST", "fallback"))
}

func TestGetEnvInt_UsesSetValue(t *testing.T) {
	t.Setenv("SOME_INT_VAR_FOR_TEST", "42")
	assert.Equal(t, 42, getEnvInt("SOME_INT_VAR_FOR_TEST", 7))
}

func TestGetEnvInt_FallbackWhenUnset(t *testing.T) {
	assert.Equal(t, 7, getEnvInt("SOME_INT_VAR_FOR_TEST_NOT_SET", 7))
}

func TestGetEnvInt_FallbackWhenInvalid(t *testing.T) {
	t.Setenv("SOME_INVALID_INT_VAR_FOR_TEST", "abc")
	assert.Equal(t, 7, getEnvInt("SOME_INVALID_INT_VAR_FOR_TEST", 7))
}
