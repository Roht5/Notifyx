package middleware

import (
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rohit-bagade/notifyx/pkg/logger"
)

// RequestLogger returns an Echo middleware that logs every request with Zap.
// Logs: method, path, status, latency, request ID.
func RequestLogger(log *logger.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)
			latency := time.Since(start)

			req := c.Request()
			res := c.Response()

			log.Infow("http request",
				"method", req.Method,
				"path", req.URL.Path,
				"status", res.Status,
				"latency_ms", latency.Milliseconds(),
				"request_id", c.Response().Header().Get(echo.HeaderXRequestID),
			)

			return err
		}
	}
}
