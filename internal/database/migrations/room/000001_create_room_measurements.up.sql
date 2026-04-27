-- Create room_measurements table
CREATE TABLE IF NOT EXISTS room_measurements (
    id BIGSERIAL PRIMARY KEY,
    timestamp TIMESTAMPTZ NOT NULL,
    device_id BIGINT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    temperature DECIMAL(5,2),
    humidity DECIMAL(5,2),
    endpoint INT
);

-- Create indexes for room_measurements
CREATE INDEX IF NOT EXISTS idx_room_measurements_timestamp ON room_measurements(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_room_measurements_device_id ON room_measurements(device_id, timestamp DESC);
