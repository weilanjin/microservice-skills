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
	"sync"
	"time"
)

const timeFormat = "2006-01-02 15:04:05.000"

// Format 表示日志输出格式。
type Format string

const (
	FormatText Format = "text"
	FormatJSON Format = "json"
)

// Output 表示一个日志输出端。
type Output struct {
	Writer io.Writer
	Format Format
	Level  slog.Leveler // 最低输出级别
}

// Config 保存日志初始化配置。
type Config struct {
	Level      slog.Leveler // slog.LevelDebug < slog.LevelInfo < slog.LevelWarn < slog.LevelError < LevelFatal
	Outputs    []Output
	AddSource  bool
	Color      bool
	Format     Format
	SourceRoot string
}

// Init 初始化 slog.Logger，设置为默认 logger 并返回。
func Init(opts ...Option) *slog.Logger {
	cfg := &Config{
		Level:      slog.LevelInfo,
		Outputs:    []Output{{Writer: os.Stdout}},
		AddSource:  true,
		Color:      true,
		Format:     FormatText,
		SourceRoot: defaultSourceRoot(),
	}
	for _, opt := range opts {
		opt(cfg)
	}

	baseHandler := newBaseHandler(cfg)
	handler := &contextHandler{Handler: baseHandler} // 支持打印传入 context 的值

	l := slog.New(handler)
	slog.SetDefault(l)
	return l
}

func newBaseHandler(cfg *Config) slog.Handler {
	outputs := normalizeOutputs(cfg)
	handlers := make([]slog.Handler, 0, len(outputs))
	for _, output := range outputs {
		handlers = append(handlers, newOutputHandler(cfg, output))
	}
	if len(handlers) == 1 {
		return handlers[0]
	}
	return slog.NewMultiHandler(handlers...)
}

func newOutputHandler(cfg *Config, output Output) slog.Handler {
	level := output.Level
	if level == nil {
		level = cfg.Level
	}
	format := output.Format
	if format == "" {
		format = cfg.Format
	}

	color := cfg.Color
	if format == FormatJSON {
		color = false
	} else if color { // 如果启用了颜色输出, 检查当前输出端是否可以使用颜色
		color = shouldUseColor(output.Writer)
	}

	options := &slog.HandlerOptions{
		Level:     level,
		AddSource: cfg.AddSource,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			return replaceAttr(color, cfg.SourceRoot, a)
		},
	}

	var handler slog.Handler
	if format == FormatJSON {
		handler = slog.NewJSONHandler(output.Writer, options) // json 格式输出
	} else {
		// text 格式输出
		handler = &consoleHandler{
			w:           output.Writer,
			mu:          new(sync.Mutex),
			level:       level,
			addSource:   cfg.AddSource,
			replaceAttr: options.ReplaceAttr,
			color:       color,
			sourceRoot:  cfg.SourceRoot,
		}
	}
	return handler
}

func normalizeOutputs(cfg *Config) []Output {
	outputs := make([]Output, 0, len(cfg.Outputs))
	for _, output := range cfg.Outputs {
		if output.Writer == nil {
			continue
		}
		outputs = append(outputs, output)
	}
	if len(outputs) == 0 {
		return []Output{{Writer: os.Stdout}}
	}
	return outputs
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

// levelText 格式化日志级别，确保 fatal 输出为 FATAL 而不是 ERROR+N。
func levelText(level slog.Level) string {
	return strings.ToUpper(level.String())
}

// logWithSource 构造带调用方源码位置的日志记录。
// callerSkip 表示需要额外跳过的调用栈层数。
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
	default:
		return slog.LevelInfo, false
	}
}
