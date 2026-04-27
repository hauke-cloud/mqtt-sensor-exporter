-- Migration: Normalize room_measurements table
-- This migration extracts device and link quality data into separate tables

-- Step 1: Create common tables if they don't exist
CREATE TABLE IF NOT EXISTS devices (
    id BIGSERIAL PRIMARY KEY,
    device_id VARCHAR(255) NOT NULL UNIQUE,
    device_name VARCHAR(255),
    short_addr VARCHAR(50),
    ieee_addr VARCHAR(100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_devices_device_id ON devices(device_id);
CREATE INDEX IF NOT EXISTS idx_devices_short_addr ON devices(short_addr);
CREATE INDEX IF NOT EXISTS idx_devices_ieee_addr ON devices(ieee_addr);

CREATE TABLE IF NOT EXISTS link_qualities (
    id BIGSERIAL PRIMARY KEY,
    timestamp TIMESTAMPTZ NOT NULL,
    device_id BIGINT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    link_quality INT
);

CREATE INDEX IF NOT EXISTS idx_link_qualities_timestamp ON link_qualities(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_link_qualities_device_id ON link_qualities(device_id, timestamp DESC);

-- Step 2: Check if old schema exists
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 
        FROM information_schema.columns 
        WHERE table_name = 'room_measurements' 
        AND column_name = 'device_id' 
        AND data_type = 'character varying'
    ) THEN
        -- Extract unique devices
        INSERT INTO devices (device_id, device_name, short_addr, ieee_addr, created_at, updated_at)
        SELECT DISTINCT 
            device_id,
            device_name,
            short_addr,
            ieee_addr,
            MIN(timestamp) as created_at,
            MAX(timestamp) as updated_at
        FROM room_measurements
        WHERE device_id IS NOT NULL
        GROUP BY device_id, device_name, short_addr, ieee_addr
        ON CONFLICT (device_id) DO UPDATE SET
            device_name = COALESCE(EXCLUDED.device_name, devices.device_name),
            short_addr = COALESCE(EXCLUDED.short_addr, devices.short_addr),
            ieee_addr = COALESCE(EXCLUDED.ieee_addr, devices.ieee_addr),
            updated_at = GREATEST(devices.updated_at, EXCLUDED.updated_at);
        
        -- Extract link quality data
        INSERT INTO link_qualities (timestamp, device_id, link_quality)
        SELECT 
            r.timestamp,
            d.id,
            r.link_quality
        FROM room_measurements r
        INNER JOIN devices d ON r.device_id = d.device_id
        WHERE r.link_quality IS NOT NULL
        ON CONFLICT DO NOTHING;
        
        -- Create new table
        CREATE TABLE room_measurements_new (
            id BIGSERIAL PRIMARY KEY,
            timestamp TIMESTAMPTZ NOT NULL,
            device_id BIGINT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
            temperature DECIMAL(5,2),
            humidity DECIMAL(5,2),
            endpoint INT
        );
        
        CREATE INDEX idx_room_measurements_new_timestamp ON room_measurements_new(timestamp DESC);
        CREATE INDEX idx_room_measurements_new_device_id ON room_measurements_new(device_id, timestamp DESC);
        
        -- Migrate data
        INSERT INTO room_measurements_new (id, timestamp, device_id, temperature, humidity, endpoint)
        SELECT 
            r.id,
            r.timestamp,
            d.id as device_id,
            r.temperature,
            r.humidity,
            r.endpoint
        FROM room_measurements r
        INNER JOIN devices d ON r.device_id = d.device_id;
        
        DROP TABLE room_measurements CASCADE;
        ALTER TABLE room_measurements_new RENAME TO room_measurements;
        ALTER INDEX idx_room_measurements_new_timestamp RENAME TO idx_room_measurements_timestamp;
        ALTER INDEX idx_room_measurements_new_device_id RENAME TO idx_room_measurements_device_id;
        
        SELECT setval('room_measurements_id_seq', (SELECT MAX(id) FROM room_measurements));
    END IF;
END $$;
