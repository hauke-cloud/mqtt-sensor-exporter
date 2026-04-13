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

// MoistureHandler handles moisture sensor measurements storage
type MoistureHandler struct {
	db  *gorm.DB
	log *zap.Logger
}

// NewMoistureHandler creates a new moisture handler
func NewMoistureHandler(db *gorm.DB, log *zap.Logger) (*MoistureHandler, error) {
	handler := &MoistureHandler{
		db:  db,
		log: log,
	}

	// Auto-migrate the schema
	if err := db.AutoMigrate(&MoistureMeasurement{}); err != nil {
		return nil, fmt.Errorf("failed to migrate moisture_measurements table: %w", err)
	}

	log.Info("Moisture handler initialized, table auto-migrated")
	return handler, nil
}

// StoreMeasurement stores a moisture measurement from a Tasmota ZbReceived message
// Example payload:
//
//	{
//	  "Device": "0xBF16",
//	  "Name": "water_moisture_2",
//	  "Temperature": 24.5,
//	  "Humidity": 0,
//	  "Endpoint": 1,
//	  "LinkQuality": 0
//	}
func (h *MoistureHandler) StoreMeasurement(ctx context.Context, deviceID string, payload map[string]any) error {
	measurement := &MoistureMeasurement{
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

	// Humidity (soil moisture)
	if humidity, ok := payload["Humidity"].(float64); ok {
		measurement.Humidity = &humidity
	}

	// Battery voltage
	if battVolt, ok := payload["BatteryVoltage"].(float64); ok {
		measurement.BatteryVoltage = &battVolt
	}

	// Battery percentage
	if battPct, ok := payload["BatteryPercentage"].(float64); ok {
		pct := int(battPct)
		measurement.BatteryPercentage = &pct
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
		h.log.Error("Failed to store moisture measurement",
			zap.String("deviceID", deviceID),
			zap.String("shortAddr", measurement.ShortAddr),
			zap.Error(err))
		return fmt.Errorf("failed to store moisture measurement: %w", err)
	}

	h.log.Debug("Stored moisture measurement",
		zap.String("deviceID", deviceID),
		zap.String("deviceName", measurement.DeviceName),
		zap.String("shortAddr", measurement.ShortAddr),
		zap.Float64p("temperature", measurement.Temperature),
		zap.Float64p("humidity", measurement.Humidity))

	return nil
}

// GetLatestMeasurement retrieves the latest measurement for a device
func (h *MoistureHandler) GetLatestMeasurement(ctx context.Context, deviceID string) (*MoistureMeasurement, error) {
	var measurement MoistureMeasurement
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
func (h *MoistureHandler) GetMeasurementsByTimeRange(ctx context.Context, deviceID string, start, end time.Time) ([]MoistureMeasurement, error) {
	var measurements []MoistureMeasurement
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
func (h *MoistureHandler) DeleteOldMeasurements(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)

	result := h.db.WithContext(ctx).
		Where("timestamp < ?", cutoff).
		Delete(&MoistureMeasurement{})

	if result.Error != nil {
		return 0, fmt.Errorf("failed to delete old measurements: %w", result.Error)
	}

	h.log.Info("Deleted old moisture measurements",
		zap.Int64("count", result.RowsAffected),
		zap.Time("cutoff", cutoff))

	return result.RowsAffected, nil
}
