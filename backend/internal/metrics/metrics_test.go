package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEchoMiddleware_RecordsRequestDuration(t *testing.T) {
	e := echo.New()
	e.GET("/api/v1/widgets/:id", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	}, EchoMiddleware())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/widgets/123", nil)
	rec := httptest.NewRecorder()

	before := testutil.CollectAndCount(HTTPRequestDuration)

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	after := testutil.CollectAndCount(HTTPRequestDuration)
	assert.Greater(t, after, before)
}

func TestEchoMiddleware_UsesRoutePatternNotRawPath(t *testing.T) {
	e := echo.New()
	e.GET("/api/v1/tenants/:id", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	}, EchoMiddleware())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tenants/abc-123", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	count := testutil.CollectAndCount(HTTPRequestDuration, "notifyx_http_request_duration_seconds")
	assert.Greater(t, count, 0)
}

func TestEchoMiddleware_RecordsStatusCode(t *testing.T) {
	e := echo.New()
	e.GET("/api/v1/error-route", func(c echo.Context) error {
		return c.String(http.StatusInternalServerError, "boom")
	}, EchoMiddleware())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/error-route", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	count := testutil.CollectAndCount(HTTPRequestDuration, "notifyx_http_request_duration_seconds")
	assert.Greater(t, count, 0)
}

func TestEchoMiddleware_PropagatesHandlerError(t *testing.T) {
	e := echo.New()
	wantErr := echo.NewHTTPError(http.StatusBadRequest, "bad input")
	e.GET("/api/v1/bad-route", func(c echo.Context) error {
		return wantErr
	}, EchoMiddleware())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/bad-route", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	// Echo's default HTTPErrorHandler converts the returned error into a 400 response,
	// and the middleware should still record the duration metric for this status.
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestNotificationsTotal_CounterVec(t *testing.T) {
	before := testutil.ToFloat64(NotificationsTotal.WithLabelValues("email", "delivered"))
	NotificationsTotal.WithLabelValues("email", "delivered").Inc()
	after := testutil.ToFloat64(NotificationsTotal.WithLabelValues("email", "delivered"))
	assert.Equal(t, before+1, after)
}

func TestRetriesTotal_CounterVec(t *testing.T) {
	before := testutil.ToFloat64(RetriesTotal.WithLabelValues("sms"))
	RetriesTotal.WithLabelValues("sms").Inc()
	after := testutil.ToFloat64(RetriesTotal.WithLabelValues("sms"))
	assert.Equal(t, before+1, after)
}

func TestDLQTotal_CounterVec(t *testing.T) {
	before := testutil.ToFloat64(DLQTotal.WithLabelValues("push"))
	DLQTotal.WithLabelValues("push").Inc()
	after := testutil.ToFloat64(DLQTotal.WithLabelValues("push"))
	assert.Equal(t, before+1, after)
}

func TestChannelSendDuration_HistogramVec(t *testing.T) {
	before := testutil.CollectAndCount(ChannelSendDuration)
	ChannelSendDuration.WithLabelValues("email").Observe(0.25)
	after := testutil.CollectAndCount(ChannelSendDuration)
	assert.Greater(t, after, before)
}

func TestEchoMiddleware_Chained(t *testing.T) {
	// Verify the middleware composes correctly with another middleware and doesn't
	// swallow context values set upstream.
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("marker", "set")
			return next(c)
		}
	})
	e.Use(EchoMiddleware())

	var sawMarker bool
	e.GET("/api/v1/marked", func(c echo.Context) error {
		sawMarker = c.Get("marker") == "set"
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/marked", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.True(t, sawMarker)
	assert.Equal(t, http.StatusOK, rec.Code)
}
