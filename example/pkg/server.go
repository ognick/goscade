package pkg

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

type HttpServer struct {
	*http.Server
}

func NewServer(addr string, handler http.Handler) *HttpServer {
	return &HttpServer{Server: &http.Server{
		Addr:    addr,
		Handler: handler,
	}}
}

func (s *HttpServer) waitForReady(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(1 * time.Millisecond):
		conn, err := net.Dial("tcp", s.Addr)
		if err != nil {
			return err
		}
		return conn.Close()
	}
}

func (s *HttpServer) Run(ctx context.Context, readinessProbe func(error)) error {
	done := make(chan error)
	go func() {
		if err := s.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			done <- fmt.Errorf("error occurred while running http server: %w", err)
		}
		close(done)
	}()

	go func() {
		err := s.waitForReady(ctx)
		readinessProbe(err)
	}()

	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("failed to listen: %v", err)
		}
	case <-ctx.Done():
		if err := s.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown http server: %w", err)
		}
	}

	return nil
}
