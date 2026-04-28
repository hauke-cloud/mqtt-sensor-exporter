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
	"strconv"
	"time"

	databaseiotgorm "github.com/hauke-cloud/database-iot-gorm"
	iotv1alpha1 "github.com/hauke-cloud/kubernetes-iot-api/api/v1alpha1"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AlertsService handles fetching devices with triggered alerts
type AlertsService struct {
	k8sClient client.Client
	db        *gorm.DB
	log       *zap.Logger
}

// NewAlertsService creates a new alerts service
func NewAlertsService(k8sClient client.Client, db *gorm.DB, log *zap.Logger) *AlertsService {
	return &AlertsService{
		k8sClient: k8sClient,
		db:        db,
		log:       log,
	}
}

// GetTriggeredAlerts returns all devices that have triggered their alert thresholds
func (s *AlertsService) GetTriggeredAlerts(ctx context.Context, filters AlertFilters) ([]AlertDevice, error) {
	// Get all devices from Kubernetes that have alert conditions
	deviceList := &iotv1alpha1.DeviceList{}
	if err := s.k8sClient.List(ctx, deviceList); err != nil {
		s.log.Error("Failed to list devices", zap.Error(err))
		return nil, fmt.Errorf("failed to list devices: %w", err)
	}

	var alertDevices []AlertDevice
	sinceTime := time.Time{}
	if filters.Since > 0 {
		sinceTime = time.Now().Add(-filters.Since)
	}

	for _, device := range deviceList.Items {
		// Skip if device has no alert condition configured
		if device.Spec.AlertCondition == nil {
			continue
		}

		// Skip if device is disabled
		if device.Spec.Disabled {
			continue
		}

		// Apply filters early to avoid unnecessary database queries
		if filters.DeviceName != "" && device.Name != filters.DeviceName {
			continue
		}

		if filters.DeviceType != "" && device.Spec.SensorType != filters.DeviceType {
			continue
		}

		if filters.Location != "" && device.Spec.Location != filters.Location {
			continue
		}

		if filters.Room != "" && device.Spec.Room != filters.Room {
			continue
		}

		// Get the latest measurement for this device
		dbDevice, currentValue, lastMeasurement, err := s.getLatestMeasurement(ctx, device.Name, device.Spec.SensorType, device.Spec.AlertCondition.Measurement)
		if err != nil {
			s.log.Warn("Failed to get latest measurement for device",
				zap.String("device", device.Name),
				zap.Error(err))
			// Skip this device - we can't evaluate the alert without measurement data
			continue
		}

		// Skip if no measurement value available
		if currentValue == nil {
			s.log.Debug("No measurement value for device",
				zap.String("device", device.Name),
				zap.String("measurement", device.Spec.AlertCondition.Measurement))
			continue
		}

		// Apply time filter based on last measurement
		if filters.Since > 0 && lastMeasurement != nil {
			if lastMeasurement.Before(sinceTime) {
				continue
			}
		}

		// Evaluate the alert condition against the current measurement value
		alertTriggered := s.evaluateAlertCondition(*currentValue, device.Spec.AlertCondition)
		if !alertTriggered {
			// Alert condition not met, skip this device
			continue
		}

		// Alert is triggered - add to results
		alertDevice := AlertDevice{
			DeviceID:        device.Name,
			DeviceName:      device.Spec.FriendlyName,
			SensorType:      device.Spec.SensorType,
			Location:        device.Spec.Location,
			Room:            device.Spec.Room,
			IEEEAddr:        device.Spec.IEEEAddr,
			CurrentValue:    currentValue,
			LastMeasurement: lastMeasurement,
			AlertCondition: AlertConditionInfo{
				Measurement: device.Spec.AlertCondition.Measurement,
				Operator:    device.Spec.AlertCondition.Operator,
				Threshold:   device.Spec.AlertCondition.Value,
			},
		}

		// Add short address from database if available
		if dbDevice != nil {
			alertDevice.ShortAddr = dbDevice.ShortAddr
		}

		alertDevices = append(alertDevices, alertDevice)
	}

	s.log.Info("Retrieved triggered alerts",
		zap.Int("count", len(alertDevices)),
		zap.String("device_name_filter", filters.DeviceName),
		zap.String("device_type_filter", filters.DeviceType),
		zap.Duration("since_filter", filters.Since))

	return alertDevices, nil
}

// getLatestMeasurement retrieves the latest measurement value for a device
func (s *AlertsService) getLatestMeasurement(ctx context.Context, deviceID, sensorType, measurementField string) (*databaseiotgorm.Device, *float64, *time.Time, error) {
	// First get the device from database
	var dbDevice databaseiotgorm.Device
	if err := s.db.WithContext(ctx).Where("device_id = ?", deviceID).First(&dbDevice).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil, nil, fmt.Errorf("device not found in database")
		}
		return nil, nil, nil, fmt.Errorf("failed to query device: %w", err)
	}

	// Query the latest measurement based on sensor type
	var value *float64
	var timestamp *time.Time

	switch sensorType {
	case "moisture":
		var measurement databaseiotgorm.MoistureMeasurement
		err := s.db.WithContext(ctx).
			Where("device_id = ?", dbDevice.ID).
			Order("timestamp DESC").
			First(&measurement).Error
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				return &dbDevice, nil, nil, nil
			}
			return &dbDevice, nil, nil, err
		}
		timestamp = &measurement.Timestamp
		value = s.extractMeasurementValue(&measurement, measurementField)

	case "valve":
		var measurement databaseiotgorm.ValveMeasurement
		err := s.db.WithContext(ctx).
			Where("device_id = ?", dbDevice.ID).
			Order("timestamp DESC").
			First(&measurement).Error
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				return &dbDevice, nil, nil, nil
			}
			return &dbDevice, nil, nil, err
		}
		timestamp = &measurement.Timestamp
		value = s.extractMeasurementValue(&measurement, measurementField)

	case "water_level":
		var measurement databaseiotgorm.WaterLevelMeasurement
		err := s.db.WithContext(ctx).
			Where("device_id = ?", dbDevice.ID).
			Order("timestamp DESC").
			First(&measurement).Error
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				return &dbDevice, nil, nil, nil
			}
			return &dbDevice, nil, nil, err
		}
		timestamp = &measurement.Timestamp
		value = s.extractMeasurementValue(&measurement, measurementField)

	case "room":
		var measurement databaseiotgorm.RoomMeasurement
		err := s.db.WithContext(ctx).
			Where("device_id = ?", dbDevice.ID).
			Order("timestamp DESC").
			First(&measurement).Error
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				return &dbDevice, nil, nil, nil
			}
			return &dbDevice, nil, nil, err
		}
		timestamp = &measurement.Timestamp
		value = s.extractMeasurementValue(&measurement, measurementField)

	default:
		return &dbDevice, nil, nil, fmt.Errorf("unsupported sensor type: %s", sensorType)
	}

	return &dbDevice, value, timestamp, nil
}

// extractMeasurementValue extracts a specific field value from a measurement struct
func (s *AlertsService) extractMeasurementValue(measurement interface{}, fieldName string) *float64 {
	switch fieldName {
	case "temperature", "Temperature":
		switch m := measurement.(type) {
		case *databaseiotgorm.MoistureMeasurement:
			if m.Temperature != nil {
				return m.Temperature
			}
		case *databaseiotgorm.RoomMeasurement:
			if m.Temperature != nil {
				return m.Temperature
			}
		}

	case "humidity", "Humidity":
		switch m := measurement.(type) {
		case *databaseiotgorm.MoistureMeasurement:
			if m.Humidity != nil {
				return m.Humidity
			}
		case *databaseiotgorm.RoomMeasurement:
			if m.Humidity != nil {
				return m.Humidity
			}
		}

	case "level", "Level":
		if m, ok := measurement.(*databaseiotgorm.WaterLevelMeasurement); ok {
			if m.Level != nil {
				val := float64(*m.Level)
				return &val
			}
		}

	case "power", "Power":
		if m, ok := measurement.(*databaseiotgorm.ValveMeasurement); ok {
			if m.Power != nil {
				val := float64(*m.Power)
				return &val
			}
		}
	}

	return nil
}

// evaluateAlertCondition checks if an alert condition is met for a given measurement value
func (s *AlertsService) evaluateAlertCondition(measurementValue float64, condition *iotv1alpha1.AlertCondition) bool {
	if condition == nil {
		return false
	}

	// Parse the threshold value from string
	thresholdValue, err := strconv.ParseFloat(condition.Value, 64)
	if err != nil {
		s.log.Warn("Failed to parse alert threshold value",
			zap.String("value", condition.Value),
			zap.Error(err))
		return false
	}

	// Evaluate based on operator
	switch condition.Operator {
	case "above":
		return measurementValue > thresholdValue
	case "below":
		return measurementValue < thresholdValue
	case "is", "equals":
		return measurementValue == thresholdValue
	default:
		s.log.Warn("Unknown alert operator",
			zap.String("operator", condition.Operator))
		return false
	}
}
