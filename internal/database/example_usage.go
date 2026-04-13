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

// Example usage of the database package with moisture sensors
//
// This file demonstrates how to use the database package to store
// moisture sensor measurements from Tasmota MQTT messages.

/*
Example 1: Initialize Database Manager
======================================

import (
	"github.com/hauke-cloud/mqtt-sensor-exporter/internal/database"
	"go.uber.org/zap"
)

// In your controller or main function:
func setupDatabaseManager(client client.Client) *database.Manager {
	zapLog, _ := zap.NewDevelopment()
	dbManager := database.NewManager(client, zapLog)
	return dbManager
}

Example 2: Connect to Database
==============================

import (
	"context"
	mqttv1alpha1 "github.com/hauke-cloud/mqtt-sensor-exporter/api/v1alpha1"
)

func connectToDatabase(ctx context.Context, dbManager *database.Manager, db *mqttv1alpha1.Database) error {
	// Database CR example:
	// apiVersion: mqtt.hauke.cloud/v1alpha1
	// kind: Database
	// metadata:
	//   name: irrigation-db
	// spec:
	//   host: "timescaledb.local"
	//   port: 5432
	//   database: "irrigation_sensors"
	//   username: "irrigation_writer"
	//   passwordSecretRef:
	//     name: irrigation-creds
	//   supportedSensorTypes:
	//     - "moisture"
	//     - "valve"

	return dbManager.Connect(ctx, db)
}

Example 3: Store Moisture Measurement from Tasmota
=================================================

func handleTasmotaMoistureMessage(ctx context.Context, dbManager *database.Manager, deviceID string) error {
	// MQTT message from Tasmota (tele/tasmota_C01240/SENSOR):
	// {
	//   "ZbReceived": {
	//     "0xBF16": {
	//       "Device": "0xBF16",
	//       "Name": "water_moisture_2",
	//       "Temperature": 24.5,
	//       "Humidity": 0,
	//       "Endpoint": 1,
	//       "LinkQuality": 0
	//     }
	//   }
	// }

	// Extract the device data from ZbReceived
	payload := map[string]any{
		"Device":      "0xBF16",
		"Name":        "water_moisture_2",
		"Temperature": 24.5,
		"Humidity":    0.0,
		"Endpoint":    1.0,
		"LinkQuality": 0.0,
	}

	// Store the measurement
	return dbManager.StoreMeasurement(ctx, deviceID, "moisture", payload)
}

Example 4: Integration with Device Controller
============================================

In internal/controller/device_controller.go or tasmota handler:

func (r *DeviceReconciler) handleMeasurement(
	ctx context.Context,
	device *mqttv1alpha1.Device,
	payload map[string]any,
) error {
	// Check if device has a sensor type
	if device.Spec.SensorType == "" {
		return nil // Skip devices without sensor type
	}

	// Store measurement using database manager
	return r.dbManager.StoreMeasurement(
		ctx,
		device.Name,
		device.Spec.SensorType,
		payload,
	)
}

Example 5: Complete Integration Flow
===================================

1. Device CR with sensor type:
   apiVersion: mqtt.hauke.cloud/v1alpha1
   kind: Device
   metadata:
     name: moisture-sensor-garden
   spec:
     ieeeAddr: "0xFFFFB40E0601BF16"
     sensorType: "moisture"
     friendlyName: "water_moisture_2"

2. Database CR:
   apiVersion: mqtt.hauke.cloud/v1alpha1
   kind: Database
   metadata:
     name: irrigation-db
   spec:
     host: "timescaledb.default.svc.cluster.local"
     port: 5432
     database: "irrigation_sensors"
     username: "irrigation_writer"
     passwordSecretRef:
       name: irrigation-creds
     supportedSensorTypes:
       - "moisture"

3. MQTT message arrives at tele/tasmota_C01240/SENSOR
4. Tasmota handler extracts ZbReceived data
5. Device matched by status.shortAddr == "0xBF16"
6. Measurement stored via dbManager.StoreMeasurement()
7. GORM creates record in moisture_measurements table:
   {
     "timestamp": "2026-04-13T21:00:00Z",
     "device_id": "moisture-sensor-garden",
     "device_name": "water_moisture_2",
     "short_addr": "0xBF16",
     "temperature": 24.5,
     "humidity": 0,
     "endpoint": 1,
     "link_quality": 0
   }

Example 6: Query Latest Measurement (Future Use)
==============================================

// This would be used in a custom controller or API
func getLatestMoistureReading(ctx context.Context, moistureHandler *database.MoistureHandler, deviceID string) error {
	measurement, err := moistureHandler.GetLatestMeasurement(ctx, deviceID)
	if err != nil {
		return err
	}

	if measurement != nil {
		log.Printf("Latest moisture reading: %.2f%%, temp: %.2f°C",
			*measurement.Humidity,
			*measurement.Temperature)
	}
	return nil
}

Example 7: Cleanup Old Data
==========================

// Run periodically (e.g., daily cron job) to clean up old measurements
func cleanupOldMeasurements(ctx context.Context, moistureHandler *database.MoistureHandler) error {
	// Delete measurements older than 90 days
	count, err := moistureHandler.DeleteOldMeasurements(ctx, 90 * 24 * time.Hour)
	if err != nil {
		return err
	}

	log.Printf("Deleted %d old measurements", count)
	return nil
}

Database Schema
===============

The moisture_measurements table will be auto-created by GORM with this schema:

CREATE TABLE moisture_measurements (
    id                SERIAL PRIMARY KEY,
    timestamp         TIMESTAMP NOT NULL,
    device_id         VARCHAR(255) NOT NULL,
    device_name       VARCHAR(255),
    short_addr        VARCHAR(50),
    ieee_addr         VARCHAR(100),
    temperature       DECIMAL(5,2),
    humidity          DECIMAL(5,2),
    battery_voltage   DECIMAL(4,2),
    battery_percentage INTEGER,
    link_quality      INTEGER,
    endpoint          INTEGER,

    INDEX idx_moisture_measurements_timestamp (timestamp),
    INDEX idx_moisture_measurements_device_id (device_id),
    INDEX idx_moisture_measurements_short_addr (short_addr),
    INDEX idx_moisture_measurements_ieee_addr (ieee_addr)
);

-- For TimescaleDB, you can convert to a hypertable:
SELECT create_hypertable('moisture_measurements', 'timestamp');

*/
