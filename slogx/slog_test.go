package slogx_test

import (
	"context"
	"log/slog"
	"misroservice/slogx"
	"testing"
	"time"
)

func init() {
	slogx.Init(slogx.WithSource())
}

func TestSlog(t *testing.T) {
	ctx := slogx.WithValue(context.Background(), "trace_id", time.Now().Unix())

	slog.DebugContext(ctx, "text slogs init", "a", 0)
	slog.InfoContext(ctx, "test slogx init", "a", 1)
	slog.WarnContext(ctx, "test slogx init", "a", 3)
	slog.Error("test slogx init", "a", 2)

	// output:
	// 2026-06-29 23:04:20.047 [INFO] slog_test.go:20 test slogx init a=1 trace_id=1782745460
	// 2026-06-29 23:04:20.047 [WARN] slog_test.go:21 test slogx init a=3 trace_id=1782745460
	// 2026-06-29 23:04:20.047 [ERROR] slog_test.go:22 test slogx init a=2
}
