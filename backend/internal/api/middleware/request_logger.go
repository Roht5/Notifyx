package middleware

import (
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rohit-bagade/notifyx/pkg/logger"
)

// RequestLogger returns an Echo middleware that logs every request with Zap.
// Logs: method, path, status, latency, request ID, and (on failure) the error.
func RequestLogger(log *logger.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)

			// Handlers in this codebase always commit their own response via c.JSON, so
			// res.Status already reflects err by the time we get here. But errors Echo's
			// router generates itself — 404s, 405s — bypass every handler and arrive here
			// uncommitted; res.Status would still read its zero value (200) without this.
			// c.Error is a no-op if the response was already committed (it checks
			// Response().Committed internally), so this is safe to call unconditionally.
			if err != nil {
				c.Error(err)
			}

			latency := time.Since(start)
			req := c.Request()
			res := c.Response()
			requestID := res.Header().Get(echo.HeaderXRequestID)

			if err != nil {
				log.Errorw("http request failed",
					"method", req.Method,
					"path", req.URL.Path,
					"status", res.Status,
					"latency_ms", latency.Milliseconds(),
					"request_id", requestID,
					"error", err.Error(),
				)
				return err
			}

			log.Infow("http request",
				"method", req.Method,
				"path", req.URL.Path,
				"status", res.Status,
				"latency_ms", latency.Milliseconds(),
				"request_id", requestID,
			)
			return nil
		}
	}
}
