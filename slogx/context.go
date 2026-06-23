package slogx

import (
	"context"
	"log/slog"
)

type ctxKey struct{}

// WithValues attaches structured attrs to context for automatic logging.
func WithValues(ctx context.Context, attrs ...slog.Attr) context.Context {
	if len(attrs) == 0 {
		return ctx
	}
	existing := valuesFromContext(ctx)
	merged := make([]slog.Attr, 0, len(existing)+len(attrs))
	merged = append(merged, existing...)
	merged = append(merged, attrs...)
	return context.WithValue(ctx, ctxKey{}, merged)
}

// WithValue attaches a single key/value into context for automatic logging.
func WithValue(ctx context.Context, key string, val any) context.Context {
	if key == "" {
		return ctx
	}
	return WithValues(ctx, slog.Any(key, val))
}

func valuesFromContext(ctx context.Context) []slog.Attr {
	if ctx == nil {
		return nil
	}
	if v, ok := ctx.Value(ctxKey{}).([]slog.Attr); ok {
		return v
	}
	return nil
}

type contextHandler struct {
	slog.Handler
}

func (h *contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if values := valuesFromContext(ctx); len(values) > 0 {
		r.AddAttrs(values...)
	}
	return h.Handler.Handle(ctx, r)
}
