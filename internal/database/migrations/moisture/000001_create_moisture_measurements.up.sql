-- Create moisture_measurements table
CREATE TABLE IF NOT EXISTS moisture_measurements (
    id BIGSERIAL PRIMARY KEY,
    timestamp TIMESTAMPTZ NOT NULL,
    device_id BIGINT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    temperature DECIMAL(5,2),
    humidity DECIMAL(5,2),
    endpoint INT
);

-- Create indexes for moisture_measurements
CREATE INDEX IF NOT EXISTS idx_moisture_measurements_timestamp ON moisture_measurements(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_moisture_measurements_device_id ON moisture_measurements(device_id, timestamp DESC);
