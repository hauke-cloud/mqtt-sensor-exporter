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

package database

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// RoomHandler handles room sensor measurements storage
type RoomHandler struct {
	db  *gorm.DB
	log *zap.Logger
}

// NewRoomHandler creates a new room handler
func NewRoomHandler(db *gorm.DB, log *zap.Logger) *RoomHandler {
	return &RoomHandler{
		db:  db,
		log: log,
	}
}

// Initialize sets up the room handler and auto-migrates the schema
func (h *RoomHandler) Initialize(ctx context.Context) error {
	// Auto-migrate the schema
	if err := h.db.AutoMigrate(&RoomMeasurement{}); err != nil {
		return fmt.Errorf("failed to migrate room_measurements table: %w", err)
	}

	h.log.Info("Room handler initialized, table auto-migrated")
	return nil
}

// StoreMeasurement stores a room measurement from a Tasmota ZbReceived message
// Example payload:
//
//	{
//	  "Device": "0xB3CC",
//	  "Temperature": 27.38,
//	  "Humidity": 51.08,
//	  "Endpoint": 1,
//	  "LinkQuality": 54
//	}
func (h *RoomHandler) StoreMeasurement(ctx context.Context, deviceID string, payload map[string]any) error {
	measurement := &RoomMeasurement{
		Timestamp: time.Now(),
		DeviceID:  deviceID,
	}

	// Extract fields from payload
	if name, ok := payload["Name"].(string); ok {
		measurement.DeviceName = name
	}

	if device, ok := payload["Device"].(string); ok {
		measurement.ShortAddr = device
	}

	// Temperature
	if temp, ok := payload["Temperature"].(float64); ok {
		measurement.Temperature = &temp
	}

	// Humidity
	if humidity, ok := payload["Humidity"].(float64); ok {
		measurement.Humidity = &humidity
	}

	// Link quality
	if lq, ok := payload["LinkQuality"].(float64); ok {
		quality := int(lq)
		measurement.LinkQuality = &quality
	}

	// Endpoint
	if ep, ok := payload["Endpoint"].(float64); ok {
		endpoint := int(ep)
		measurement.Endpoint = &endpoint
	}

	// Store in database
	if err := h.db.WithContext(ctx).Create(measurement).Error; err != nil {
		h.log.Error("Failed to store room measurement",
			zap.String("deviceID", deviceID),
			zap.String("shortAddr", measurement.ShortAddr),
			zap.Error(err))
		return fmt.Errorf("failed to store room measurement: %w", err)
	}

	h.log.Debug("Stored room measurement",
		zap.String("deviceID", deviceID),
		zap.String("deviceName", measurement.DeviceName),
		zap.String("shortAddr", measurement.ShortAddr),
		zap.Float64p("temperature", measurement.Temperature),
		zap.Float64p("humidity", measurement.Humidity))

	return nil
}

// GetLatestMeasurement retrieves the latest measurement for a device
func (h *RoomHandler) GetLatestMeasurement(ctx context.Context, deviceID string) (*RoomMeasurement, error) {
	var measurement RoomMeasurement
	err := h.db.WithContext(ctx).
		Where("device_id = ?", deviceID).
		Order("timestamp DESC").
		First(&measurement).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get latest measurement: %w", err)
	}

	return &measurement, nil
}

// GetMeasurementsByTimeRange retrieves measurements within a time range
func (h *RoomHandler) GetMeasurementsByTimeRange(ctx context.Context, deviceID string, start, end time.Time) ([]RoomMeasurement, error) {
	var measurements []RoomMeasurement
	err := h.db.WithContext(ctx).
		Where("device_id = ? AND timestamp BETWEEN ? AND ?", deviceID, start, end).
		Order("timestamp ASC").
		Find(&measurements).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get measurements by time range: %w", err)
	}

	return measurements, nil
}

// DeleteOldMeasurements deletes measurements older than the specified duration
func (h *RoomHandler) DeleteOldMeasurements(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)

	result := h.db.WithContext(ctx).
		Where("timestamp < ?", cutoff).
		Delete(&RoomMeasurement{})

	if result.Error != nil {
		return 0, fmt.Errorf("failed to delete old measurements: %w", result.Error)
	}

	h.log.Info("Deleted old room measurements",
		zap.Int64("count", result.RowsAffected),
		zap.Time("cutoff", cutoff))

	return result.RowsAffected, nil
}
