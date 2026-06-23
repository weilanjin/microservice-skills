package slogx

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const timeFormat = "2006-01-02 15:04:05.000"

// LevelFatal defines a custom fatal level above error.
// slog 标准库没有 Fatal 级别，这里扩展一个更高的级别用于致命错误。
const LevelFatal = slog.Level(12)

// Fatal 以致命级别输出日志后直接退出进程（exit code 1）。
func Fatal(ctx context.Context, msg string, args ...any) {
	logWithSource(ctx, LevelFatal, msg, args, 1)
}

// Format indicates output format.
type Format string

const (
	FormatText Format = "text"
	FormatJSON Format = "json"
)

// Config holds logger setup options.
type Config struct {
	Level      slog.Leveler // slog.LevelDebug < slog.LevelInfo < slog.LevelWarn < slog.LevelError < LevelFatal
	Writer     io.Writer
	AddSource  bool
	Color      bool
	Format     Format
	SourceRoot string
}

// Init builds a slog Logger with colorized console output and source location,
// sets it as the default logger, and returns it.
func Init(opts ...Option) *slog.Logger {
	cfg := &Config{
		Level:      slog.LevelInfo,
		Writer:     os.Stdout,
		AddSource:  true,
		Color:      true,
		Format:     FormatText,
		SourceRoot: defaultSourceRoot(),
	}
	for _, opt := range opts {
		opt(cfg)
	}

	if cfg.Color { // 如果启用了颜色输出, 检查是否可以使用颜色
		cfg.Color = shouldUseColor(cfg.Writer)
	}

	baseHandler := newBaseHandler(cfg)
	handler := &contextHandler{Handler: baseHandler} // 支持打印传入 context 的值

	l := slog.New(handler)
	slog.SetDefault(l)
	return l
}

func newBaseHandler(cfg *Config) slog.Handler {
	options := &slog.HandlerOptions{
		Level:     cfg.Level,
		AddSource: cfg.AddSource,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			return replaceAttr(cfg.Color, cfg.SourceRoot, a)
		},
	}

	if cfg.Format == FormatJSON {
		cfg.Color = false
		return slog.NewJSONHandler(cfg.Writer, options) // json 格式输出
	}

	// text 格式输出
	return &consoleHandler{
		w:           cfg.Writer,
		level:       cfg.Level,
		addSource:   cfg.AddSource,
		replaceAttr: options.ReplaceAttr,
		color:       cfg.Color,
		sourceRoot:  cfg.SourceRoot,
	}
}

func replaceAttr(enableColor bool, sourceRoot string, a slog.Attr) slog.Attr {
	switch a.Key {
	case slog.LevelKey:
		level, ok := valueToLevel(a.Value)
		if !ok {
			return a
		}
		upper := levelText(level)
		if enableColor {
			upper = colorize(level, upper)
		}
		a.Value = slog.StringValue(upper)
	case slog.TimeKey:
		if t, ok := valueToTime(a.Value); ok {
			a.Value = slog.StringValue(t.Local().Format(timeFormat))
		}
	case slog.SourceKey:
		if src, ok := a.Value.Any().(slog.Source); ok {
			file := trimSourcePath(sourceRoot, src.File)
			a.Value = slog.StringValue(fmt.Sprintf("%s:%d", file, src.Line))
		}
	}
	return a
}

func valueToLevel(v slog.Value) (slog.Level, bool) {
	switch v.Kind() {
	case slog.KindInt64:
		return slog.Level(v.Int64()), true
	case slog.KindString:
		return parseLevel(v.String())
	default:
		if lv, ok := v.Any().(slog.Level); ok {
			return lv, true
		}
		return slog.LevelInfo, false
	}
}

func valueToTime(v slog.Value) (time.Time, bool) {
	switch v.Kind() {
	case slog.KindTime:
		return v.Time(), true
	default:
		if t, ok := v.Any().(time.Time); ok {
			return t, true
		}
		return time.Time{}, false
	}
}

// 是否可以使用颜色输出
func shouldUseColor(w io.Writer) bool {
	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	if (info.Mode() & os.ModeCharDevice) == 0 {
		return false
	}
	return true
}

func trimSourcePath(root, file string) string {
	if root != "" {
		if rel, err := filepath.Rel(root, file); err == nil && !strings.HasPrefix(rel, "..") {
			return rel
		}
	}
	return file
}

const (
	colorReset   = "\033[0m"
	colorRed     = "\033[31m" // 红色
	colorMagenta = "\033[35m" // 紫红色
	colorYellow  = "\033[33m" // 黄色
	colorGreen   = "\033[32m" // 绿色
	colorBlue    = "\033[34m" // 蓝色
	colorCyan    = "\033[36m" // 蓝绿色
)

// slog.Level 颜色
func colorize(level slog.Level, text string) string {
	switch {
	case level >= LevelFatal:
		return colorMagenta + text + colorReset
	case level >= slog.LevelError:
		return colorRed + text + colorReset
	case level >= slog.LevelWarn:
		return colorYellow + text + colorReset
	case level >= slog.LevelInfo:
		return colorGreen + text + colorReset
	default:
		return colorBlue + text + colorReset
	}
}

// levelText normalizes level to display text, ensuring fatal renders as FATAL instead of ERROR+N.
func levelText(level slog.Level) string {
	if level >= LevelFatal {
		return "FATAL"
	}
	return strings.ToUpper(level.String())
}

// logWithSource builds a record with caller PC to retain correct source location when called via wrappers.
// callerSkip is the additional stack frames to skip above this helper (e.g., wrapper functions).
func logWithSource(ctx context.Context, level slog.Level, msg string, args []any, callerSkip int) {
	h := slog.Default().Handler()

	pcs := make([]uintptr, 16)
	n := runtime.Callers(2+callerSkip, pcs) // skip runtime.Callers + logWithSource + wrapper(s)
	frames := runtime.CallersFrames(pcs[:n])
	var pc uintptr
	for {
		frame, more := frames.Next()
		if frame.File != "" && !strings.Contains(frame.File, "/runtime/") {
			pc = frame.PC
			break
		}
		if !more {
			break
		}
	}
	rec := slog.NewRecord(time.Now(), level, msg, pc)
	rec.Add(args...)
	if err := h.Handle(ctx, rec); err != nil {
		slog.WarnContext(ctx, "log with source", "rec", rec, "err", err)
	}
}

func defaultSourceRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return wd
}

func parseLevel(v string) (slog.Level, bool) {
	switch strings.ToLower(v) {
	case "debug":
		return slog.LevelDebug, true
	case "info":
		return slog.LevelInfo, true
	case "warn":
		return slog.LevelWarn, true
	case "error":
		return slog.LevelError, true
	case "fatal":
		return LevelFatal, true
	default:
		return slog.LevelInfo, false
	}
}
