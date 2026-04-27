-- Create valve_measurements table
CREATE TABLE IF NOT EXISTS valve_measurements (
    id BIGSERIAL PRIMARY KEY,
    timestamp TIMESTAMPTZ NOT NULL,
    device_id BIGINT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    power INT,
    last_valve_open_duration INT,
    irrigation_start_time BIGINT,
    irrigation_end_time BIGINT,
    daily_irrigation_volume INT,
    endpoint INT
);

-- Create indexes for valve_measurements
CREATE INDEX IF NOT EXISTS idx_valve_measurements_timestamp ON valve_measurements(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_valve_measurements_device_id ON valve_measurements(device_id, timestamp DESC);
