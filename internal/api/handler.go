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
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/hauke-cloud/iot-api/alerts"
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

// HandleAlerts handles GET /api/v2/alerts and GET /api/v2/alerts/{device-name}
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

	// Parse filters from query parameters and path
	filters, err := h.parseAlertFilters(r)
	if err != nil {
		h.log.Warn("Invalid filter parameters", zap.Error(err))
		h.sendError(w, fmt.Sprintf("Invalid filter parameters: %v", err), http.StatusBadRequest)
		return
	}

	h.log.Info("Processing alerts request",
		zap.String("client", clientCN),
		zap.String("remote_addr", r.RemoteAddr),
		zap.String("device_name", filters.DeviceName),
		zap.String("device_type", filters.DeviceType),
		zap.Duration("since", filters.Since))

	// Get triggered alerts with filters
	devices, err := h.alertsService.GetTriggeredAlerts(r.Context(), filters)
	if err != nil {
		h.log.Error("Failed to get triggered alerts", zap.Error(err))
		h.sendError(w, "Failed to retrieve alerts", http.StatusInternalServerError)
		return
	}

	// If filtering by device name and no results, return 404
	if filters.DeviceName != "" && len(devices) == 0 {
		h.sendError(w, fmt.Sprintf("Device '%s' not found or has no active alerts", filters.DeviceName), http.StatusNotFound)
		return
	}

	// Build response
	response := alerts.AlertsResponse{
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

// parseAlertFilters parses query parameters and path parameters into AlertFilters
func (h *Handler) parseAlertFilters(r *http.Request) (alerts.AlertFilters, error) {
	filters := alerts.AlertFilters{}

	// Extract device name from path if present: /api/v2/alerts/{device-name}
	path := r.URL.Path
	if strings.HasPrefix(path, "/api/v2/alerts/") {
		deviceName := strings.TrimPrefix(path, "/api/v2/alerts/")
		if deviceName != "" && deviceName != "/" {
			filters.DeviceName = strings.Trim(deviceName, "/")
		}
	}

	// Parse query parameters
	query := r.URL.Query()

	// device-type filter
	if deviceType := query.Get("device-type"); deviceType != "" {
		filters.DeviceType = deviceType
	}

	// location filter
	if location := query.Get("location"); location != "" {
		filters.Location = location
	}

	// room filter
	if room := query.Get("room"); room != "" {
		filters.Room = room
	}

	// since filter - parse duration
	if sinceStr := query.Get("since"); sinceStr != "" {
		duration, err := parseDuration(sinceStr)
		if err != nil {
			return filters, fmt.Errorf("invalid 'since' parameter: %w", err)
		}
		filters.Since = duration
	}

	return filters, nil
}

// parseDuration parses duration strings like "1min", "5m", "1h", "30s"
func parseDuration(s string) (time.Duration, error) {
	// Handle common abbreviations
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "min", "m")
	s = strings.ReplaceAll(s, "hour", "h")
	s = strings.ReplaceAll(s, "sec", "s")
	s = strings.ReplaceAll(s, "day", "h")

	// For days, convert to hours
	if strings.HasSuffix(s, "d") {
		daysStr := strings.TrimSuffix(s, "d")
		var days int
		if _, err := fmt.Sscanf(daysStr, "%d", &days); err != nil {
			return 0, err
		}
		return time.Duration(days*24) * time.Hour, nil
	}

	return time.ParseDuration(s)
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
	json.NewEncoder(w).Encode(alerts.ErrorResponse{
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
