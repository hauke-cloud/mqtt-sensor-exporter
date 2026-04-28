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
func (s *AlertsService) GetTriggeredAlerts(ctx context.Context) ([]AlertDevice, error) {
	// Get all devices from Kubernetes that have alert conditions and are in alert state
	deviceList := &iotv1alpha1.DeviceList{}
	if err := s.k8sClient.List(ctx, deviceList); err != nil {
		s.log.Error("Failed to list devices", zap.Error(err))
		return nil, fmt.Errorf("failed to list devices: %w", err)
	}

	var alertDevices []AlertDevice

	for _, device := range deviceList.Items {
		// Skip if device has no alert condition configured
		if device.Spec.AlertCondition == nil {
			continue
		}

		// Skip if device is disabled
		if device.Spec.Disabled {
			continue
		}

		// Check if alert is currently triggered in the status
		if !device.Status.Alert {
			continue
		}

		// Get the latest measurement for this device
		dbDevice, currentValue, lastMeasurement, err := s.getLatestMeasurement(ctx, device.Name, device.Spec.SensorType, device.Spec.AlertCondition.Measurement)
		if err != nil {
			s.log.Warn("Failed to get latest measurement for device",
				zap.String("device", device.Name),
				zap.Error(err))
			// Continue to next device - we still want to report the alert even without measurement data
		}

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
		zap.Int("count", len(alertDevices)))

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
