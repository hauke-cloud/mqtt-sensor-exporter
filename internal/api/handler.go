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
	"encoding/json"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// Handler handles HTTP requests for the API
type Handler struct {
	alertsService *AlertsService
	log           *zap.Logger
}

// NewHandler creates a new API handler
func NewHandler(alertsService *AlertsService, log *zap.Logger) *Handler {
	return &Handler{
		alertsService: alertsService,
		log:           log,
	}
}

// HandleAlerts handles GET /v2/api/alerts
func (h *Handler) HandleAlerts(w http.ResponseWriter, r *http.Request) {
	// Only allow GET requests
	if r.Method != http.MethodGet {
		h.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get client certificate info for logging
	var clientCN string
	if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 {
		clientCN = r.TLS.PeerCertificates[0].Subject.CommonName
	}

	h.log.Info("Processing alerts request",
		zap.String("client", clientCN),
		zap.String("remote_addr", r.RemoteAddr))

	// Get triggered alerts
	devices, err := h.alertsService.GetTriggeredAlerts(r.Context())
	if err != nil {
		h.log.Error("Failed to get triggered alerts", zap.Error(err))
		h.sendError(w, "Failed to retrieve alerts", http.StatusInternalServerError)
		return
	}

	// Build response
	response := AlertsResponse{
		Devices:   devices,
		Count:     len(devices),
		Timestamp: time.Now(),
	}

	// Send JSON response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.log.Error("Failed to encode response", zap.Error(err))
	}

	h.log.Info("Alerts request completed",
		zap.Int("alert_count", len(devices)),
		zap.String("client", clientCN))
}

// HandleHealth handles GET /health
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	})
}

// sendError sends an error response
func (h *Handler) sendError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(ErrorResponse{
		Error:     message,
		Code:      code,
		Timestamp: time.Now(),
	})
}

// LoggingMiddleware logs all incoming requests
func (h *Handler) LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Log request
		h.log.Debug("Incoming request",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.String("remote_addr", r.RemoteAddr))

		// Call next handler
		next.ServeHTTP(w, r)

		// Log completion
		h.log.Debug("Request completed",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.Duration("duration", time.Since(start)))
	})
}
