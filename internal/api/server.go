/*
Copyright 2026 hauke.cloud.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// ServerConfig contains configuration for the API server
type ServerConfig struct {
	// Address to bind the server to (e.g., ":8080")
	Address string
}

// Server is the REST API server
type Server struct {
	config  ServerConfig
	handler *Handler
	log     *zap.Logger
	server  *http.Server
}

// NewServer creates a new API server
func NewServer(config ServerConfig, handler *Handler, log *zap.Logger) *Server {
	return &Server{
		config:  config,
		handler: handler,
		log:     log,
	}
}

// Start starts the API server
func (s *Server) Start(ctx context.Context) error {
	// Create router
	mux := http.NewServeMux()
	
	// Register routes
	mux.HandleFunc("/v2/api/alerts", s.handler.HandleAlerts)
	mux.HandleFunc("/health", s.handler.HandleHealth)

	// Wrap with logging middleware
	handler := s.handler.LoggingMiddleware(mux)

	// Create HTTP server
	s.server = &http.Server{
		Addr:    s.config.Address,
		Handler: handler,
		// Security timeouts
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	s.log.Info("Starting API server",
		zap.String("address", s.config.Address))

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Wait for context cancellation or server error
	select {
	case <-ctx.Done():
		s.log.Info("Shutting down API server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return s.server.Shutdown(shutdownCtx)
	case err := <-errChan:
		return fmt.Errorf("server error: %w", err)
	}
}

// Stop stops the API server
func (s *Server) Stop(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	return s.server.Shutdown(ctx)
}
