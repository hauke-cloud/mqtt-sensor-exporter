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
	"time"
)

// MoistureMeasurement represents a moisture sensor measurement in the database
// This model is used by GORM to auto-create/manage the moisture_measurements table
type MoistureMeasurement struct {
	ID                uint      `gorm:"primaryKey"`
	Timestamp         time.Time `gorm:"index;not null"`
	DeviceID          string    `gorm:"index;size:255;not null"` // Device CR name
	DeviceName        string    `gorm:"size:255"`                // Friendly name from Tasmota
	ShortAddr         string    `gorm:"index;size:50"`           // Zigbee short address (e.g., "0xBF16")
	IEEEAddr          string    `gorm:"index;size:100"`          // IEEE address if available
	Temperature       *float64  `gorm:"type:decimal(5,2)"`       // Temperature in Celsius
	Humidity          *float64  `gorm:"type:decimal(5,2)"`       // Soil humidity/moisture percentage
	BatteryVoltage    *float64  `gorm:"type:decimal(4,2)"`       // Battery voltage
	BatteryPercentage *int      // Battery percentage (0-100)
	LinkQuality       *int      // Link quality (0-255)
	Endpoint          *int      // Zigbee endpoint
}

// TableName overrides the default table name
func (MoistureMeasurement) TableName() string {
	return "moisture_measurements"
}

// ValveMeasurement represents a valve sensor measurement in the database
// This model is used by GORM to auto-create/manage the valve_measurements table
type ValveMeasurement struct {
	ID                    uint      `gorm:"primaryKey"`
	Timestamp             time.Time `gorm:"index;not null"`
	DeviceID              string    `gorm:"index;size:255;not null"` // Device CR name
	DeviceName            string    `gorm:"size:255"`                // Friendly name from Tasmota
	ShortAddr             string    `gorm:"index;size:50"`           // Zigbee short address
	IEEEAddr              string    `gorm:"index;size:100"`          // IEEE address if available
	Power                 *int      // Power state (0=off, 1=on)
	LastValveOpenDuration *int      // Duration valve was open (seconds)
	IrrigationStartTime   *int64    // Unix timestamp when irrigation started
	IrrigationEndTime     *int64    // Unix timestamp when irrigation ended
	DailyIrrigationVolume *int      // Daily irrigation volume
	BatteryVoltage        *float64  `gorm:"type:decimal(4,2)"` // Battery voltage
	BatteryPercentage     *int      // Battery percentage (0-100)
	LinkQuality           *int      // Link quality (0-255)
	Endpoint              *int      // Zigbee endpoint
}

// TableName overrides the default table name
func (ValveMeasurement) TableName() string {
	return "valve_measurements"
}

// WaterLevelMeasurement represents a water level sensor measurement in the database
// This model is used by GORM to auto-create/manage the water_level_measurements table
type WaterLevelMeasurement struct {
	ID                uint      `gorm:"primaryKey"`
	Timestamp         time.Time `gorm:"index;not null"`
	DeviceID          string    `gorm:"index;size:255;not null"` // Device CR name
	DeviceName        string    `gorm:"size:255"`                // Friendly name from Tasmota
	ShortAddr         string    `gorm:"index;size:50"`           // Zigbee short address
	IEEEAddr          string    `gorm:"index;size:100"`          // IEEE address if available
	Level             *int      // Water level value (e.g., 285)
	BatteryPercentage *int      // Battery percentage (0-100)
	LinkQuality       *int      // Link quality (0-255)
	Endpoint          *int      // Zigbee endpoint
}

// TableName overrides the default table name
func (WaterLevelMeasurement) TableName() string {
	return "water_level_measurements"
}

// RoomMeasurement represents a room sensor measurement in the database
// This model is used by GORM to auto-create/manage the room_measurements table
type RoomMeasurement struct {
	ID          uint      `gorm:"primaryKey"`
	Timestamp   time.Time `gorm:"index;not null"`
	DeviceID    string    `gorm:"index;size:255;not null"` // Device CR name
	DeviceName  string    `gorm:"size:255"`                // Friendly name from Tasmota
	ShortAddr   string    `gorm:"index;size:50"`           // Zigbee short address (e.g., "0xB3CC")
	IEEEAddr    string    `gorm:"index;size:100"`          // IEEE address if available
	Temperature *float64  `gorm:"type:decimal(5,2)"`       // Temperature in Celsius
	Humidity    *float64  `gorm:"type:decimal(5,2)"`       // Humidity percentage
	LinkQuality *int      // Link quality (0-255)
	Endpoint    *int      // Zigbee endpoint
}

// TableName overrides the default table name
func (RoomMeasurement) TableName() string {
	return "room_measurements"
}
