package hs

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"misroservice/hs/response"
	"misroservice/slogx"
	"net/http"
	"runtime/debug"
	"time"
)

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.ErrorContext(r.Context(), "Recovered from panic", slog.Any("error", err))
				debug.PrintStack()
				response.JSON(w, map[string]any{
					"codes": 1,
					"msg":   "Internal Server Error",
				})
			}
		}()

		now := time.Now()
		next.ServeHTTP(w, r)
		attrs := []any{
			slog.String("time", time.Since(now).String()),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.String("remote_addr", r.RemoteAddr),
		}
		slog.InfoContext(r.Context(), "Request received", attrs...)
	})
}

// RequestID injects a request id into context and response header for tracing.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-Id")
		if reqID == "" {
			reqID = newRequestID()
		}
		ctx := slogx.WithValue(r.Context(), "trace_id", reqID)
		r = r.WithContext(ctx)
		w.Header().Set("X-Request-Id", reqID)
		next.ServeHTTP(w, r)
	})
}

func newRequestID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	return hex.EncodeToString(b[:])
}

// Cors 设置跨域请求所需的响应头
func Cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 设置CORS响应头
		origin := r.Header.Get("Origin")
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,AccessToken,X-CSRF-Token, Authorization, Token,X-Token,X-User-Id")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS,DELETE,PUT")
		w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers, Content-Type, New-Token, New-Expires-At")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Max-Age", "86400")

		// 继续处理请求
		next.ServeHTTP(w, r)
	})
}
