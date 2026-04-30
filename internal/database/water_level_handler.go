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

	databaseiotgorm "github.com/hauke-cloud/database-iot-gorm"
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

// StoreMeasurement stores a water level measurement to the database
func (h *WaterLevelHandler) StoreMeasurement(ctx context.Context, deviceID string, payload map[string]any) error {
	timestamp := time.Now()

	// Find or create device
	var device databaseiotgorm.Device
	result := h.db.WithContext(ctx).Where("device_id = ?", deviceID).First(&device)
	if result.Error == gorm.ErrRecordNotFound {
		device = databaseiotgorm.Device{
			DeviceID:   deviceID,
			SensorType: "water_level",
		}

		if name, ok := payload["Name"].(string); ok {
			device.DeviceName = sanitizeString(name)
		}
		if shortAddr, ok := payload["databaseiotgorm.Device"].(string); ok {
			device.ShortAddr = sanitizeString(shortAddr)
		}
		if ieeeAddr, ok := payload["IEEEAddr"].(string); ok {
			device.IEEEAddr = sanitizeString(ieeeAddr)
		}

		if err := h.db.WithContext(ctx).Create(&device).Error; err != nil {
			h.log.Error("Failed to create device",
				zap.String("deviceID", deviceID),
				zap.Error(err))
			return fmt.Errorf("failed to create device: %w", err)
		}
	} else if result.Error != nil {
		return fmt.Errorf("failed to query device: %w", result.Error)
	} else {
		updated := false
		if name, ok := payload["Name"].(string); ok && name != "" && device.DeviceName != name {
			device.DeviceName = sanitizeString(name)
			updated = true
		}
		if shortAddr, ok := payload["databaseiotgorm.Device"].(string); ok && shortAddr != "" && device.ShortAddr != shortAddr {
			device.ShortAddr = sanitizeString(shortAddr)
			updated = true
		}
		if ieeeAddr, ok := payload["IEEEAddr"].(string); ok && ieeeAddr != "" && device.IEEEAddr != ieeeAddr {
			device.IEEEAddr = sanitizeString(ieeeAddr)
			updated = true
		}
		if updated {
			h.db.WithContext(ctx).Save(&device)
		}
	}

	measurement := databaseiotgorm.WaterLevelMeasurement{
		Timestamp: timestamp,
		DeviceID:  device.ID,
	}

	if level, ok := payload["WaterLevel"].(float64); ok {
		levelInt := int(level)
		measurement.Level = &levelInt
	}

	if endpoint, ok := payload["Endpoint"].(float64); ok {
		ep := int(endpoint)
		measurement.Endpoint = &ep
	}

	if err := h.db.WithContext(ctx).Create(&measurement).Error; err != nil {
		return fmt.Errorf("failed to store water level measurement: %w", err)
	}

	// Store battery information if present
	if bp, ok := payload["BatteryPercentage"].(float64); ok {
		pct := int(bp)
		battery := &databaseiotgorm.Battery{
			Timestamp:         timestamp,
			DeviceID:          device.ID,
			BatteryPercentage: &pct,
		}
		if err := h.db.WithContext(ctx).Create(battery).Error; err != nil {
			h.log.Warn("Failed to store battery measurement",
				zap.String("deviceID", deviceID),
				zap.Error(err))
		}
	}

	// Store link quality if present
	if lq, ok := payload["LinkQuality"].(float64); ok {
		quality := int(lq)
		linkQuality := &databaseiotgorm.LinkQuality{
			Timestamp:   timestamp,
			DeviceID:    device.ID,
			LinkQuality: &quality,
		}
		if err := h.db.WithContext(ctx).Create(linkQuality).Error; err != nil {
			h.log.Warn("Failed to store link quality",
				zap.String("deviceID", deviceID),
				zap.Error(err))
		}
	}

	h.log.Debug("Stored water level measurement",
		zap.String("deviceID", deviceID),
		zap.Any("level", measurement.Level))

	return nil
}
