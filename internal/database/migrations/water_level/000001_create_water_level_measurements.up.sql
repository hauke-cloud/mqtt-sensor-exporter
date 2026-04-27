-- Create water_level_measurements table
CREATE TABLE IF NOT EXISTS water_level_measurements (
    id BIGSERIAL PRIMARY KEY,
    timestamp TIMESTAMPTZ NOT NULL,
    device_id BIGINT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    level INT,
    endpoint INT
);

-- Create indexes for water_level_measurements
CREATE INDEX IF NOT EXISTS idx_water_level_measurements_timestamp ON water_level_measurements(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_water_level_measurements_device_id ON water_level_measurements(device_id, timestamp DESC);
