package slogx

import (
	"io"
	"log/slog"
)

// Option customizes logger Config.
type Option func(*Config)

// WithLevel sets the log level.
func WithLevel(level slog.Leveler) Option {
	return func(c *Config) {
		c.Level = level
	}
}

// WithWriter sets the output writer.
func WithWriter(w io.Writer) Option {
	return func(c *Config) {
		if w != nil {
			c.Writer = w
		}
	}
}

// WithFormat sets output format (text/json).
func WithFormat(format Format) Option {
	return func(c *Config) {
		if format != "" {
			c.Format = format
		}
	}
}

// WithColor explicitly enables or disables colorized output.
func WithColor() Option {
	return func(c *Config) {
		c.Color = true
	}
}

// WithSource controls whether to emit source location.
func WithSource() Option {
	return func(c *Config) {
		c.AddSource = true
	}
}
