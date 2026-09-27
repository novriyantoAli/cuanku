// Package middleware holds the HTTP middleware shared by every route.
package middleware

import (
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Logger logs one structured line per request.
func Logger(logger *zap.Logger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		start := time.Now()
		path := ctx.Request.URL.Path
		query := ctx.Request.URL.RawQuery

		ctx.Next()

		fields := []zap.Field{
			zap.String("method", ctx.Request.Method),
			zap.String("path", path),
			zap.String("query", query),
			zap.Int("status", ctx.Writer.Status()),
			zap.Duration("latency", time.Since(start)),
			zap.String("client_ip", ctx.ClientIP()),
		}
		if len(ctx.Errors) > 0 {
			fields = append(fields, zap.String("errors", ctx.Errors.ByType(gin.ErrorTypePrivate).String()))
		}

		switch {
		case ctx.Writer.Status() >= http.StatusInternalServerError:
			logger.Error("http request", fields...)
		case ctx.Writer.Status() >= http.StatusBadRequest:
			logger.Warn("http request", fields...)
		default:
			logger.Info("http request", fields...)
		}
	}
}

// Recovery converts a panic into a 500 without leaking the stack trace to
// the client, logging it instead.
func Recovery(logger *zap.Logger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("panic recovered",
					zap.Any("panic", recovered),
					zap.String("path", ctx.Request.URL.Path),
					zap.ByteString("stack", debug.Stack()),
				)
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			}
		}()

		ctx.Next()
	}
}

// CORS allows the configured browser origins (the SvelteKit dev server by
// default) to call the API. Origins are matched exactly; "*" is only honoured
// when explicitly configured, and never combined with credentials.
func CORS(allowedOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	allowAll := false
	for _, origin := range allowedOrigins {
		if origin == "*" {
			allowAll = true
		}
		allowed[strings.ToLower(origin)] = struct{}{}
	}

	return func(ctx *gin.Context) {
		origin := ctx.GetHeader("Origin")
		if origin != "" {
			if _, ok := allowed[strings.ToLower(origin)]; ok || allowAll {
				ctx.Header("Access-Control-Allow-Origin", origin)
				ctx.Header("Vary", "Origin")
				ctx.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				ctx.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
				ctx.Header("Access-Control-Max-Age", strconv.Itoa(int((12 * time.Hour).Seconds())))
			}
		}

		if ctx.Request.Method == http.MethodOptions {
			ctx.AbortWithStatus(http.StatusNoContent)
			return
		}

		ctx.Next()
	}
}
