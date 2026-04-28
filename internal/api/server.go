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
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"time"

	"go.uber.org/zap"
)

// ServerConfig contains configuration for the API server
type ServerConfig struct {
	// Address to bind the server to (e.g., ":8443")
	Address string

	// TLSCertPath is the path to the server TLS certificate
	TLSCertPath string

	// TLSKeyPath is the path to the server TLS key
	TLSKeyPath string

	// ClientCAPath is the path to the CA certificate for validating client certificates
	ClientCAPath string

	// RequireClientCert determines if client certificates are required
	RequireClientCert bool
}

// Server is the REST API server with client certificate authentication
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

// Start starts the API server with TLS and client certificate authentication
func (s *Server) Start(ctx context.Context) error {
	// Set up TLS configuration
	tlsConfig, err := s.setupTLS()
	if err != nil {
		return fmt.Errorf("failed to setup TLS: %w", err)
	}

	// Create router
	mux := http.NewServeMux()
	
	// Register routes
	mux.HandleFunc("/v2/api/alerts", s.handler.HandleAlerts)
	mux.HandleFunc("/health", s.handler.HandleHealth)

	// Wrap with logging middleware
	handler := s.handler.LoggingMiddleware(mux)

	// Create HTTP server
	s.server = &http.Server{
		Addr:      s.config.Address,
		Handler:   handler,
		TLSConfig: tlsConfig,
		// Security timeouts
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	s.log.Info("Starting API server",
		zap.String("address", s.config.Address),
		zap.Bool("client_cert_required", s.config.RequireClientCert))

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := s.server.ListenAndServeTLS(s.config.TLSCertPath, s.config.TLSKeyPath); err != nil && err != http.ErrServerClosed {
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

// setupTLS configures TLS with client certificate authentication
func (s *Server) setupTLS() (*tls.Config, error) {
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		},
		PreferServerCipherSuites: true,
	}

	// Configure client certificate authentication
	if s.config.ClientCAPath != "" {
		// Load CA certificate for validating client certificates
		caCert, err := os.ReadFile(s.config.ClientCAPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read client CA certificate: %w", err)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse client CA certificate")
		}

		tlsConfig.ClientCAs = caCertPool

		// Set client certificate policy
		if s.config.RequireClientCert {
			tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
			s.log.Info("Client certificate authentication enabled",
				zap.String("mode", "required"))
		} else {
			tlsConfig.ClientAuth = tls.VerifyClientCertIfGiven
			s.log.Info("Client certificate authentication enabled",
				zap.String("mode", "optional"))
		}
	} else {
		s.log.Warn("No client CA path provided, client certificate authentication disabled")
		tlsConfig.ClientAuth = tls.NoClientCert
	}

	return tlsConfig, nil
}
