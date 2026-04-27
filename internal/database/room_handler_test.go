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
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestRoomHandler_StoreMeasurement(t *testing.T) {
	// Create mock database
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock database: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	// Create GORM DB with mock
	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: db,
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create GORM DB: %v", err)
	}

	// Create handler
	log := zap.NewNop()
	handler := NewRoomHandler(gormDB, log)

	// Mock expectations for finding/creating device
	mock.ExpectQuery(`SELECT \* FROM "devices" WHERE device_id`).
		WithArgs("test-device", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	// Mock device creation
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "devices"`).
		WithArgs(
			"test-device",    // device_id
			"",               // device_name
			"0xB3CC",         // short_addr
			"",               // ieee_addr
			sqlmock.AnyArg(), // created_at
			sqlmock.AnyArg(), // updated_at
		).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectCommit()

	// Mock measurement insertion
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "room_measurements"`).
		WithArgs(
			sqlmock.AnyArg(), // timestamp
			1,                // device_id (foreign key)
			27.38,            // temperature
			51.08,            // humidity
			1,                // endpoint
		).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectCommit()

	// Mock link quality insertion
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "link_qualities"`).
		WithArgs(
			sqlmock.AnyArg(), // timestamp
			1,                // device_id
			54,               // link_quality
		).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectCommit()

	// Test payload matching the example from requirements
	payload := map[string]any{
		"Device":      "0xB3CC",
		"Temperature": 27.38,
		"Humidity":    51.08,
		"Endpoint":    1.0,
		"LinkQuality": 54.0,
	}

	ctx := context.Background()
	err = handler.StoreMeasurement(ctx, "test-device", payload)
	if err != nil {
		t.Errorf("StoreMeasurement failed: %v", err)
	}

	// Verify all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestRoomHandler_GetLatestMeasurement(t *testing.T) {
	// Create mock database
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock database: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	// Create GORM DB with mock
	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: db,
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create GORM DB: %v", err)
	}

	// Create handler
	log := zap.NewNop()
	handler := NewRoomHandler(gormDB, log)

	// Mock expectations for SELECT device
	deviceRows := sqlmock.NewRows([]string{
		"id", "device_id", "device_name", "short_addr", "ieee_addr", "created_at", "updated_at",
	}).AddRow(
		1, "test-device", "room-sensor", "0xB3CC", "", time.Now(), time.Now(),
	)

	mock.ExpectQuery(`SELECT \* FROM "devices" WHERE device_id`).
		WithArgs("test-device", 1).
		WillReturnRows(deviceRows)

	// Mock expectations for SELECT measurement (no preload now)
	now := time.Now()
	temp := 27.38
	humidity := 51.08
	endpoint := 1

	measurementRows := sqlmock.NewRows([]string{
		"id", "timestamp", "device_id", "temperature", "humidity", "endpoint",
	}).AddRow(
		1, now, 1, temp, humidity, endpoint,
	)

	mock.ExpectQuery(`SELECT \* FROM "room_measurements" WHERE device_id`).
		WithArgs(1, 1).
		WillReturnRows(measurementRows)

	ctx := context.Background()
	measurement, err := handler.GetLatestMeasurement(ctx, "test-device")
	if err != nil {
		t.Errorf("GetLatestMeasurement failed: %v", err)
	}

	if measurement == nil {
		t.Fatal("Expected measurement, got nil")
	}

	if measurement.Device.DeviceID != "test-device" {
		t.Errorf("Expected DeviceID 'test-device', got '%s'", measurement.Device.DeviceID)
	}

	if measurement.Temperature == nil || *measurement.Temperature != 27.38 {
		t.Errorf("Expected Temperature 27.38, got %v", measurement.Temperature)
	}

	if measurement.Humidity == nil || *measurement.Humidity != 51.08 {
		t.Errorf("Expected Humidity 51.08, got %v", measurement.Humidity)
	}

	// Verify all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestRoomHandler_DeleteOldMeasurements(t *testing.T) {
	// Create mock database
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock database: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	// Create GORM DB with mock
	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: db,
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create GORM DB: %v", err)
	}

	// Create handler
	log := zap.NewNop()
	handler := NewRoomHandler(gormDB, log)

	// Mock expectations for DELETE
	mock.ExpectBegin()
	mock.ExpectExec(`DELETE FROM "room_measurements" WHERE timestamp < \$1`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 10))
	mock.ExpectCommit()

	ctx := context.Background()
	deleted, err := handler.DeleteOldMeasurements(ctx, 24*time.Hour)
	if err != nil {
		t.Errorf("DeleteOldMeasurements failed: %v", err)
	}

	if deleted != 10 {
		t.Errorf("Expected 10 deleted rows, got %d", deleted)
	}

	// Verify all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}
