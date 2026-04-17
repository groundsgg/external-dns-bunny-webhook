package health

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"
)

type Options struct {
	Host         string        `env:"HOST, default=0.0.0.0"`
	Port         string        `env:"PORT, default=8080"`
	ReadTimeout  time.Duration `env:"READ_TIMEOUT, default=60s"`
	WriteTimeout time.Duration `env:"WRITE_TIMEOUT, default=60s"`
}

func (o *Options) Addr() string {
	return fmt.Sprintf("%s:%s", o.Host, o.Port)
}

type Server struct {
	Options Options
	healthy atomic.Bool
}

func (s *Server) SetHealthy(healthy bool) {
	switch healthy {
	case true:
		slog.Info("Service is now healthy.")
	default:
		slog.Warn("Service is unhealthy.")
	}
	s.healthy.Store(healthy)
}

func (s *Server) Healthy() bool {
	return s.healthy.Load()
}

func (s *Server) Serve(ctx context.Context) error {
	m := http.NewServeMux()
	m.HandleFunc("/healthz", s.handleHealthz)

	srv := &http.Server{
		Addr:         s.Options.Addr(),
		Handler:      m,
		ReadTimeout:  s.Options.ReadTimeout,
		WriteTimeout: s.Options.WriteTimeout,
	}

	shutdownErr := make(chan error, 1)
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		shutdownErr <- srv.Shutdown(shutdownCtx)
	}()

	err := srv.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("health server: %w", err)
	}

	if err := <-shutdownErr; err != nil {
		return fmt.Errorf("health server shutdown: %w", err)
	}
	return nil
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	if s.Healthy() {
		writeResponse(w, http.StatusOK, "Healthy")
		return
	}
	writeResponse(w, http.StatusServiceUnavailable, "Not Healthy")
}

func writeResponse(w http.ResponseWriter, status int, message string) {
	w.WriteHeader(status)
	_, _ = w.Write([]byte(message))
}
