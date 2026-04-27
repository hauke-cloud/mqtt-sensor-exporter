-- Create devices table (shared across all sensor types)
CREATE TABLE IF NOT EXISTS devices (
    id BIGSERIAL PRIMARY KEY,
    device_id VARCHAR(255) NOT NULL UNIQUE,
    device_name VARCHAR(255),
    short_addr VARCHAR(50),
    ieee_addr VARCHAR(100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for devices
CREATE INDEX IF NOT EXISTS idx_devices_device_id ON devices(device_id);
CREATE INDEX IF NOT EXISTS idx_devices_short_addr ON devices(short_addr);
CREATE INDEX IF NOT EXISTS idx_devices_ieee_addr ON devices(ieee_addr);

-- Create batteries table (shared across sensor types that report battery)
CREATE TABLE IF NOT EXISTS batteries (
    id BIGSERIAL PRIMARY KEY,
    timestamp TIMESTAMPTZ NOT NULL,
    device_id BIGINT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    battery_percentage INT
);

-- Create indexes for batteries
CREATE INDEX IF NOT EXISTS idx_batteries_timestamp ON batteries(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_batteries_device_id ON batteries(device_id, timestamp DESC);

-- Create link_qualities table (shared across all sensor types)
CREATE TABLE IF NOT EXISTS link_qualities (
    id BIGSERIAL PRIMARY KEY,
    timestamp TIMESTAMPTZ NOT NULL,
    device_id BIGINT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    link_quality INT
);

-- Create indexes for link_qualities
CREATE INDEX IF NOT EXISTS idx_link_qualities_timestamp ON link_qualities(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_link_qualities_device_id ON link_qualities(device_id, timestamp DESC);
