# Database Package

This package provides GORM-based database integration for storing sensor measurements from MQTT devices.

## Features

- **GORM Integration**: Auto-creates and manages database tables
- **Type-Based Routing**: Routes measurements to appropriate handlers based on sensor type
- **PostgreSQL/TimescaleDB Support**: Optimized for time-series data
- **Connection Pooling**: Configurable connection pools per database
- **Multi-Database Support**: Different sensor types can use different databases
- **Secure Authentication**: Supports password and client certificate authentication

## Supported Sensor Types

| Sensor Type | Table Name | Handler | Status |
|-------------|------------|---------|--------|
| `moisture` | `moisture_measurements` | `MoistureHandler` | ✅ Implemented |
| `valve` | `valve_measurements` | `ValveHandler` | 🔨 Planned |
| `power` | `power_measurements` | `PowerHandler` | 🔨 Planned |
| `solar` | `solar_measurements` | `SolarHandler` | 🔨 Planned |

## Architecture

```
Device with sensorType="moisture"
  ↓
Database Manager (finds matching database)
  ↓
Moisture Handler
  ↓
GORM (creates/manages table)
  ↓
PostgreSQL/TimescaleDB
```

## Moisture Sensor Implementation

### Data Model

The `MoistureMeasurement` struct maps to the `moisture_measurements` table:

```go
type MoistureMeasurement struct {
    ID                uint      // Auto-increment primary key
    Timestamp         time.Time // When measurement was taken
    DeviceID          string    // Device CR name
    DeviceName        string    // Friendly name from Tasmota
    ShortAddr         string    // Zigbee short address (e.g., "0xBF16")
    IEEEAddr          string    // IEEE address if available
    Temperature       *float64  // Temperature in Celsius
    Humidity          *float64  // Soil moisture/humidity percentage
    BatteryVoltage    *float64  // Battery voltage
    BatteryPercentage *int      // Battery percentage (0-100)
    LinkQuality       *int      // Link quality (0-255)
    Endpoint          *int      // Zigbee endpoint
}
```

### Tasmota Message Format

The moisture handler processes messages from Tasmota's `ZbReceived` payload:

```json
{
  "ZbReceived": {
    "0xBF16": {
      "Device": "0xBF16",
      "Name": "water_moisture_2",
      "Temperature": 24.5,
      "Humidity": 0,
      "Endpoint": 1,
      "LinkQuality": 0
    }
  }
}
```

### Usage Example

```go
import (
    "context"
    "github.com/hauke-cloud/mqtt-sensor-exporter/internal/database"
    "go.uber.org/zap"
)

// 1. Create database manager
zapLog, _ := zap.NewDevelopment()
dbManager := database.NewManager(client, zapLog)

// 2. Connect to database (from Database CR)
err := dbManager.Connect(ctx, databaseCR)

// 3. Store measurement from Tasmota
payload := map[string]any{
    "Device":      "0xBF16",
    "Name":        "water_moisture_2",
    "Temperature": 24.5,
    "Humidity":    0.0,
    "Endpoint":    1.0,
    "LinkQuality": 0.0,
}

err = dbManager.StoreMeasurement(ctx, "moisture-sensor-garden", "moisture", payload)
```

## Database Schema

GORM auto-creates this schema for moisture_measurements:

```sql
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
    endpoint          INTEGER
);

CREATE INDEX idx_moisture_measurements_timestamp ON moisture_measurements(timestamp);
CREATE INDEX idx_moisture_measurements_device_id ON moisture_measurements(device_id);
CREATE INDEX idx_moisture_measurements_short_addr ON moisture_measurements(short_addr);
CREATE INDEX idx_moisture_measurements_ieee_addr ON moisture_measurements(ieee_addr);
```

### TimescaleDB Hypertable

For optimal time-series performance, convert to hypertable:

```sql
SELECT create_hypertable('moisture_measurements', 'timestamp');
```

## API Reference

### Manager

#### `NewManager(client client.Client, log *zap.Logger) *Manager`
Creates a new database manager.

#### `Connect(ctx context.Context, database *Database) error`
Connects to a database and initializes handlers for supported sensor types.

#### `Disconnect(namespace, name string)`
Closes connection to a database.

#### `StoreMeasurement(ctx context.Context, deviceID, sensorType string, payload map[string]any) error`
Stores a measurement by routing it to the appropriate handler.

### MoistureHandler

#### `NewMoistureHandler(db *gorm.DB, log *zap.Logger) (*MoistureHandler, error)`
Creates a moisture handler and auto-migrates the table schema.

#### `StoreMeasurement(ctx context.Context, deviceID string, payload map[string]any) error`
Stores a moisture measurement from a Tasmota ZbReceived payload.

#### `GetLatestMeasurement(ctx context.Context, deviceID string) (*MoistureMeasurement, error)`
Retrieves the most recent measurement for a device.

#### `GetMeasurementsByTimeRange(ctx context.Context, deviceID string, start, end time.Time) ([]MoistureMeasurement, error)`
Retrieves measurements within a time range.

#### `DeleteOldMeasurements(ctx context.Context, olderThan time.Duration) (int64, error)`
Deletes measurements older than the specified duration.

## Files

- `manager.go` - Database connection manager and routing
- `models.go` - GORM data models for all sensor types
- `moisture_handler.go` - Moisture sensor measurement handler
- `example_usage.go` - Comprehensive usage examples

## Future Enhancements

- [ ] Valve handler for water valve measurements
- [ ] Power handler for electrical measurements
- [ ] Solar handler for solar panel measurements
- [ ] Batching support for high-volume sensors
- [ ] Automatic data retention policies
- [ ] Query APIs for retrieving historical data
- [ ] Grafana integration examples
