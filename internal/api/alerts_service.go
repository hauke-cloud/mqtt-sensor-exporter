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
	dbGetter  func() *gorm.DB
	log       *zap.Logger
}

// NewAlertsService creates a new alerts service
// dbGetter is a function that returns the current database connection (can be nil if not yet available)
func NewAlertsService(k8sClient client.Client, dbGetter func() *gorm.DB, log *zap.Logger) *AlertsService {
	return &AlertsService{
		k8sClient: k8sClient,
		dbGetter:  dbGetter,
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

		// Get the measurement value for this device
		// If 'since' filter is provided, calculate average over that time window
		dbDevice, currentValue, lastMeasurement, err := s.getMeasurementValue(ctx, device.Name, device.Spec.SensorType, device.Spec.AlertCondition.Measurement, filters.Since)
		if err != nil {
			s.log.Warn("Failed to get measurement value for device",
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

// getMeasurementValue retrieves the measurement value for a device
// Always calculates the average over a time window
// If sinceDuration is 0, defaults to -1m
func (s *AlertsService) getMeasurementValue(ctx context.Context, deviceID, sensorType, measurementField string, sinceDuration time.Duration) (*databaseiotgorm.Device, *float64, *time.Time, error) {
	// Get database connection
	db := s.dbGetter()
	if db == nil {
		return nil, nil, nil, fmt.Errorf("database not available for device_id=%s, sensor_type=%s, measurement_field=%s", deviceID, sensorType, measurementField)
	}

	// First get the device from database
	var dbDevice databaseiotgorm.Device
	if err := db.WithContext(ctx).Where("device_id = ?", deviceID).First(&dbDevice).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil, nil, fmt.Errorf("device not found in database: device_id=%s, sensor_type=%s", deviceID, sensorType)
		}
		return nil, nil, nil, fmt.Errorf("failed to query device from database: device_id=%s, sensor_type=%s, error=%w", deviceID, sensorType, err)
	}

	// Default to -1m if no duration specified
	if sinceDuration == 0 {
		sinceDuration = time.Minute
	}

	return s.getAverageMeasurement(ctx, db, &dbDevice, sensorType, measurementField, sinceDuration)
}

// getAverageMeasurement calculates the average measurement value over a time window
func (s *AlertsService) getAverageMeasurement(ctx context.Context, db *gorm.DB, dbDevice *databaseiotgorm.Device, sensorType, measurementField string, sinceDuration time.Duration) (*databaseiotgorm.Device, *float64, *time.Time, error) {
	// Calculate the time range
	sinceTime := time.Now().Add(-sinceDuration)

	var avgValue float64
	var lastTimestamp time.Time
	var err error

	switch sensorType {
	case "moisture":
		var result struct {
			AvgValue      float64
			LastTimestamp time.Time
		}

		query := db.WithContext(ctx).
			Table("moisture_measurements").
			Select("AVG("+getMeasurementColumn(measurementField)+") as avg_value, MAX(timestamp) as last_timestamp").
			Where("device_id = ? AND timestamp >= ?", dbDevice.ID, sinceTime)

		err = query.Scan(&result).Error
		avgValue = result.AvgValue
		lastTimestamp = result.LastTimestamp

	case "valve":
		var result struct {
			AvgValue      float64
			LastTimestamp time.Time
		}

		query := db.WithContext(ctx).
			Table("valve_measurements").
			Select("AVG("+getMeasurementColumn(measurementField)+") as avg_value, MAX(timestamp) as last_timestamp").
			Where("device_id = ? AND timestamp >= ?", dbDevice.ID, sinceTime)

		err = query.Scan(&result).Error
		avgValue = result.AvgValue
		lastTimestamp = result.LastTimestamp

	case "water_level":
		var result struct {
			AvgValue      float64
			LastTimestamp time.Time
		}

		query := db.WithContext(ctx).
			Table("water_level_measurements").
			Select("AVG("+getMeasurementColumn(measurementField)+") as avg_value, MAX(timestamp) as last_timestamp").
			Where("device_id = ? AND timestamp >= ?", dbDevice.ID, sinceTime)

		err = query.Scan(&result).Error
		avgValue = result.AvgValue
		lastTimestamp = result.LastTimestamp

	case "room":
		var result struct {
			AvgValue      float64
			LastTimestamp time.Time
		}

		query := db.WithContext(ctx).
			Table("room_measurements").
			Select("AVG("+getMeasurementColumn(measurementField)+") as avg_value, MAX(timestamp) as last_timestamp").
			Where("device_id = ? AND timestamp >= ?", dbDevice.ID, sinceTime)

		err = query.Scan(&result).Error
		avgValue = result.AvgValue
		lastTimestamp = result.LastTimestamp

	default:
		return dbDevice, nil, nil, fmt.Errorf("unsupported sensor type: sensor_type=%s, device_id=%s, measurement_field=%s", sensorType, dbDevice.DeviceID, measurementField)
	}

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return dbDevice, nil, nil, nil
		}
		return dbDevice, nil, nil, fmt.Errorf("failed to query average measurement: sensor_type=%s, device_id=%s, db_device_id=%d, measurement_field=%s, since=%v, error=%w", sensorType, dbDevice.DeviceID, dbDevice.ID, measurementField, sinceDuration, err)
	}

	// Check if we got valid data
	if lastTimestamp.IsZero() {
		return dbDevice, nil, nil, nil
	}

	return dbDevice, &avgValue, &lastTimestamp, nil
}

// getMeasurementColumn returns the database column name for a measurement field
func getMeasurementColumn(fieldName string) string {
	switch fieldName {
	case "temperature", "Temperature":
		return "temperature"
	case "humidity", "Humidity":
		return "humidity"
	case "level", "Level", "water_level":
		return "level"
	case "power", "Power":
		return "power"
	default:
		return fieldName
	}
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
