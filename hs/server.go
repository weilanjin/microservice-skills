package hs

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Engine struct {
	addr    string
	srv     *http.Server
	handler http.Handler
}

func New(addr string) *Engine {
	return &Engine{
		addr:    addr,
		handler: http.DefaultServeMux,
	}
}

func (e *Engine) SetHandler(handler http.Handler) {
	e.handler = handler
}

func (e *Engine) Run() error {
	e.srv = &http.Server{
		Addr:    e.addr,
		Handler: e.handler,
	}

	errCh := make(chan error, 1)

	go func() {
		slog.Info("Starting server", "address", e.addr)
		if err := e.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case <-quit:
		return e.shutdown()
	}
}

func (e *Engine) shutdown() error {
	slog.Info("Shutting down server gracefully")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := e.srv.Shutdown(ctx); err != nil {
		slog.Error("Server shutdown failed", "error", err)
		return err
	}
	slog.Info("Server stopped gracefully")
	return nil
}
