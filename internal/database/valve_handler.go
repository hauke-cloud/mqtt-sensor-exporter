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

// ValveHandler handles storage of valve sensor measurements
type ValveHandler struct {
	db  *gorm.DB
	log *zap.Logger
}

// NewValveHandler creates a new valve handler
func NewValveHandler(db *gorm.DB, log *zap.Logger) *ValveHandler {
	return &ValveHandler{
		db:  db,
		log: log,
	}
}

// Initialize creates the valve_measurements table if it doesn't exist
func (h *ValveHandler) Initialize(ctx context.Context) error {
	h.log.Info("Initializing valve_measurements table")

	// Auto-migrate the table schema
	if err := h.db.WithContext(ctx).AutoMigrate(&ValveMeasurement{}); err != nil {
		return fmt.Errorf("failed to auto-migrate valve_measurements table: %w", err)
	}

	h.log.Info("Valve measurements table initialized successfully")
	return nil
}

// StoreMeasurement stores a valve measurement to the database
func (h *ValveHandler) StoreMeasurement(ctx context.Context, deviceID string, payload map[string]any) error {
	measurement := ValveMeasurement{
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

	// Extract power state
	if power, ok := payload["Power"].(float64); ok {
		p := int(power)
		measurement.Power = &p
	}

	// Extract last valve open duration
	if duration, ok := payload["LastValveOpenDuration"].(float64); ok {
		d := int(duration)
		measurement.LastValveOpenDuration = &d
	}

	// Extract irrigation start time
	if startTime, ok := payload["IrrigationStartTime"].(float64); ok {
		st := int64(startTime)
		measurement.IrrigationStartTime = &st
	}

	// Extract irrigation end time
	if endTime, ok := payload["IrrigationEndTime"].(float64); ok {
		et := int64(endTime)
		measurement.IrrigationEndTime = &et
	}

	// Extract daily irrigation volume
	if volume, ok := payload["DailyIrrigationVolume"].(float64); ok {
		v := int(volume)
		measurement.DailyIrrigationVolume = &v
	}

	// Extract battery percentage if present
	if batteryPercentage, ok := payload["BatteryPercentage"].(float64); ok {
		bp := int(batteryPercentage)
		measurement.BatteryPercentage = &bp
	}

	// Extract battery voltage if present
	if voltage, ok := payload["Voltage"].(float64); ok {
		measurement.BatteryVoltage = &voltage
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
		return fmt.Errorf("failed to store valve measurement: %w", err)
	}

	h.log.Debug("Stored valve measurement",
		zap.String("deviceID", deviceID),
		zap.Any("power", measurement.Power),
		zap.Any("duration", measurement.LastValveOpenDuration))

	return nil
}
