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
	timestamp := time.Now()

	// Find or create device
	var device Device
	result := h.db.WithContext(ctx).Where("device_id = ?", deviceID).First(&device)
	if result.Error == gorm.ErrRecordNotFound {
		device = Device{
			DeviceID: deviceID,
		}

		if name, ok := payload["Name"].(string); ok {
			device.DeviceName = name
		}
		if shortAddr, ok := payload["Device"].(string); ok {
			device.ShortAddr = shortAddr
		}
		if ieeeAddr, ok := payload["IEEEAddr"].(string); ok {
			device.IEEEAddr = ieeeAddr
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
			device.DeviceName = name
			updated = true
		}
		if shortAddr, ok := payload["Device"].(string); ok && shortAddr != "" && device.ShortAddr != shortAddr {
			device.ShortAddr = shortAddr
			updated = true
		}
		if ieeeAddr, ok := payload["IEEEAddr"].(string); ok && ieeeAddr != "" && device.IEEEAddr != ieeeAddr {
			device.IEEEAddr = ieeeAddr
			updated = true
		}
		if updated {
			h.db.WithContext(ctx).Save(&device)
		}
	}

	measurement := &RoomMeasurement{
		Timestamp: timestamp,
		DeviceID:  device.ID,
	}

	if temp, ok := payload["Temperature"].(float64); ok {
		measurement.Temperature = &temp
	}

	if humidity, ok := payload["Humidity"].(float64); ok {
		measurement.Humidity = &humidity
	}

	if ep, ok := payload["Endpoint"].(float64); ok {
		endpoint := int(ep)
		measurement.Endpoint = &endpoint
	}

	// Store in database
	if err := h.db.WithContext(ctx).Create(measurement).Error; err != nil {
		h.log.Error("Failed to store room measurement",
			zap.String("deviceID", deviceID),
			zap.Error(err))
		return fmt.Errorf("failed to store room measurement: %w", err)
	}

	// Store link quality if present
	if lq, ok := payload["LinkQuality"].(float64); ok {
		quality := int(lq)
		linkQuality := &LinkQuality{
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

	h.log.Debug("Stored room measurement",
		zap.String("deviceID", deviceID),
		zap.String("deviceName", device.DeviceName),
		zap.Float64p("temperature", measurement.Temperature),
		zap.Float64p("humidity", measurement.Humidity))

	return nil
}

// GetLatestMeasurement retrieves the latest measurement for a device
func (h *RoomHandler) GetLatestMeasurement(ctx context.Context, deviceID string) (*RoomMeasurement, error) {
	var device Device
	if err := h.db.WithContext(ctx).Where("device_id = ?", deviceID).First(&device).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find device: %w", err)
	}

	var measurement RoomMeasurement
	err := h.db.WithContext(ctx).
		Preload("Device").
		Where("device_id = ?", device.ID).
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
	var device Device
	if err := h.db.WithContext(ctx).Where("device_id = ?", deviceID).First(&device).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return []RoomMeasurement{}, nil
		}
		return nil, fmt.Errorf("failed to find device: %w", err)
	}

	var measurements []RoomMeasurement
	err := h.db.WithContext(ctx).
		Preload("Device").
		Where("device_id = ? AND timestamp BETWEEN ? AND ?", device.ID, start, end).
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
