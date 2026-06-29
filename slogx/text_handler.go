package slogx

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type consoleHandler struct {
	w           io.Writer
	mu          *sync.Mutex
	level       slog.Leveler
	addSource   bool
	replaceAttr func([]string, slog.Attr) slog.Attr
	attrs       []slog.Attr
	groups      []string
	color       bool
	sourceRoot  string
}

func (h *consoleHandler) Enabled(_ context.Context, level slog.Level) bool {
	minLevel := slog.LevelInfo
	if h.level != nil {
		minLevel = h.level.Level()
	}
	return level >= minLevel
}

func (h *consoleHandler) Handle(ctx context.Context, r slog.Record) error {
	if !h.Enabled(ctx, r.Level) {
		return nil
	}

	var buf bytes.Buffer
	ts := r.Time
	if ts.IsZero() {
		ts = time.Now()
	}
	timeStr := ts.Local().Format(timeFormat)
	if h.color {
		timeStr = colorCyan + timeStr + colorReset
	}
	buf.WriteString(timeStr)
	buf.WriteByte(' ')

	lvl := levelText(r.Level)
	if h.color {
		lvl = colorize(r.Level, lvl)
	}
	buf.WriteByte('[')
	buf.WriteString(lvl)
	buf.WriteString("] ")

	if h.addSource {
		src := sourceFromRecord(r)
		if src.File != "" {
			path := trimSourcePath(h.sourceRoot, src.File)
			if h.color {
				path = colorBlue + path + colorReset
			}
			buf.WriteString(path)
			buf.WriteByte(':')
			lineStr := fmt.Sprintf("%d", src.Line)
			if h.color {
				lineStr = colorBlue + lineStr + colorReset
			}
			buf.WriteString(lineStr)
			buf.WriteByte(' ')
		}
	}

	msg := r.Message
	if h.color {
		msg = colorize(r.Level, msg)
	}
	buf.WriteString(msg)

	attrs := make([]slog.Attr, 0, len(h.attrs)+r.NumAttrs())
	for _, a := range h.attrs {
		attrs = appendAttr(attrs, h.groups, h.replaceAttr, a)
	}
	r.Attrs(func(a slog.Attr) bool {
		attrs = appendAttr(attrs, h.groups, h.replaceAttr, a)
		return true
	})

	for _, a := range attrs {
		key := a.Key
		val := formatValue(a.Value)
		if h.color {
			key = colorCyan + key + colorReset
			val = colorBlue + val + colorReset
		}
		buf.WriteByte(' ')
		buf.WriteString(key)
		buf.WriteByte('=')
		buf.WriteString(val)
	}

	buf.WriteByte('\n')
	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.w.Write(buf.Bytes())
	return err
}

func (h *consoleHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	merged := make([]slog.Attr, 0, len(h.attrs)+len(attrs))
	merged = append(merged, h.attrs...)
	merged = append(merged, attrs...)
	return &consoleHandler{
		w:           h.w,
		mu:          h.mu,
		level:       h.level,
		addSource:   h.addSource,
		replaceAttr: h.replaceAttr,
		attrs:       merged,
		groups:      h.groups,
		color:       h.color,
		sourceRoot:  h.sourceRoot,
	}
}

func (h *consoleHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	groups := append([]string{}, h.groups...)
	groups = append(groups, name)
	return &consoleHandler{
		w:           h.w,
		mu:          h.mu,
		level:       h.level,
		addSource:   h.addSource,
		replaceAttr: h.replaceAttr,
		attrs:       h.attrs,
		groups:      groups,
		color:       h.color,
		sourceRoot:  h.sourceRoot,
	}
}

func appendAttr(dst []slog.Attr, groups []string, replacer func([]string, slog.Attr) slog.Attr, a slog.Attr) []slog.Attr {
	if len(groups) > 0 {
		keyParts := append(append([]string{}, groups...), a.Key)
		a.Key = strings.Join(keyParts, ".")
	}
	if replacer != nil {
		a = replacer(groups, a)
	}
	if a.Equal(slog.Attr{}) {
		return dst
	}
	return append(dst, a)
}

func formatValue(v slog.Value) string {
	switch v.Kind() {
	case slog.KindString:
		return v.String()
	case slog.KindBool:
		return strconv.FormatBool(v.Bool())
	case slog.KindInt64:
		return fmt.Sprint(v.Int64())
	case slog.KindFloat64:
		return strconv.FormatFloat(v.Float64(), 'f', -1, 64)
	case slog.KindDuration:
		return v.Duration().String()
	case slog.KindTime:
		return v.Time().Local().Format(timeFormat)
	default:
		return fmt.Sprint(v.Any())
	}
}

func sourceFromRecord(r slog.Record) slog.Source {
	if src := r.Source(); src != nil && src.File != "" {
		return *src
	}
	if pc, file, line, ok := runtime.Caller(4); ok {
		return slog.Source{Function: runtime.FuncForPC(pc).Name(), File: file, Line: line}
	}
	return slog.Source{}
}
