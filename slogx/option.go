package slogx

import "log/slog"

// Option 用于自定义日志初始化配置。
type Option func(*Config)

// WithLevel 设置最低输出日志级别。
func WithLevel(level slog.Leveler) Option {
	return func(c *Config) {
		c.Level = level
	}
}

// WithFormat 设置日志输出格式，支持 text/json。
func WithFormat(format Format) Option {
	return func(c *Config) {
		if format != "" {
			c.Format = format
		}
	}
}

// WithSource 开启源码位置输出。
func WithSource() Option {
	return func(c *Config) {
		c.AddSource = true
	}
}

// WithOutput 添加一个日志输出端。 可以指定多个
func WithOutput(output Output) Option {
	return func(c *Config) {
		if output.Writer == nil {
			return
		}
		c.Outputs = append(c.Outputs, output)
	}
}
