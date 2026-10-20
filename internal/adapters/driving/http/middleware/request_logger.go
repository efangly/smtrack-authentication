package middleware

import (
	"errors"
	"log/slog"
	"time"

	"github.com/efangly/smtrack-authentication/pkg/logger"
	"github.com/gofiber/fiber/v3"
)

// RequestLogger returns a Fiber middleware that logs each request as structured JSON
// compatible with Grafana Loki.
func RequestLogger() fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()

		// Process request
		err := c.Next()

		latency := time.Since(start)

		// The error handler has not written the response yet at this point, so
		// take the status from the error itself when there is one.
		status := c.Response().StatusCode()
		if err != nil {
			status = fiber.StatusInternalServerError
			var fiberErr *fiber.Error
			if errors.As(err, &fiberErr) {
				status = fiberErr.Code
			}
		}

		attrs := []any{
			slog.String("method", c.Method()),
			slog.String("path", c.Path()),
			slog.Int("status", status),
			slog.String("latency", latency.String()),
			slog.Int64("latency_ms", latency.Milliseconds()),
			slog.String("ip", c.IP()),
			slog.String("user_agent", c.Get("User-Agent")),
		}

		if queryStr := string(c.Request().URI().QueryString()); queryStr != "" {
			attrs = append(attrs, slog.String("query", queryStr))
		}

		if err != nil {
			attrs = append(attrs, slog.String("error", err.Error()))
		}

		switch {
		case status >= 500:
			logger.Error("request completed", attrs...)
		case status >= 400:
			logger.Warn("request completed", attrs...)
		default:
			logger.Info("request completed", attrs...)
		}

		return err
	}
}
