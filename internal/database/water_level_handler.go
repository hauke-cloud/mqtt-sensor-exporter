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

// WaterLevelHandler handles storage of water level sensor measurements
type WaterLevelHandler struct {
	db  *gorm.DB
	log *zap.Logger
}

// NewWaterLevelHandler creates a new water level handler
func NewWaterLevelHandler(db *gorm.DB, log *zap.Logger) *WaterLevelHandler {
	return &WaterLevelHandler{
		db:  db,
		log: log,
	}
}

// Initialize creates the water_level_measurements table if it doesn't exist
func (h *WaterLevelHandler) Initialize(ctx context.Context) error {
	h.log.Info("Initializing water_level_measurements table")

	// Auto-migrate the table schema
	if err := h.db.WithContext(ctx).AutoMigrate(&WaterLevelMeasurement{}); err != nil {
		return fmt.Errorf("failed to auto-migrate water_level_measurements table: %w", err)
	}

	h.log.Info("Water level measurements table initialized successfully")
	return nil
}

// StoreMeasurement stores a water level measurement to the database
func (h *WaterLevelHandler) StoreMeasurement(ctx context.Context, deviceID string, payload map[string]any) error {
	measurement := WaterLevelMeasurement{
		Timestamp: time.Now(),
		DeviceID:  deviceID,
	}

	// Extract device name if present
	if name, ok := payload["Name"].(string); ok {
		measurement.DeviceName = name
	}

	// Extract device short address if present
	if device, ok := payload["Device"].(string); ok {
		measurement.ShortAddr = device
	}

	// Extract water level (primary measurement)
	if level, ok := payload["WaterLevel"].(float64); ok {
		levelInt := int(level)
		measurement.Level = &levelInt
	}

	// Extract battery percentage if present
	if batteryPercentage, ok := payload["BatteryPercentage"].(float64); ok {
		bp := int(batteryPercentage)
		measurement.BatteryPercentage = &bp
	}

	// Extract link quality if present
	if linkQuality, ok := payload["LinkQuality"].(float64); ok {
		lq := int(linkQuality)
		measurement.LinkQuality = &lq
	}

	// Extract endpoint if present
	if endpoint, ok := payload["Endpoint"].(float64); ok {
		ep := int(endpoint)
		measurement.Endpoint = &ep
	}

	// Store to database
	if err := h.db.WithContext(ctx).Create(&measurement).Error; err != nil {
		return fmt.Errorf("failed to store water level measurement: %w", err)
	}

	h.log.Debug("Stored water level measurement",
		zap.String("deviceID", deviceID),
		zap.Any("level", measurement.Level))

	return nil
}
