-- Migration: Normalize moisture_measurements table
-- This migration extracts device, battery, and link quality data into separate tables

-- Step 1: Create common tables if they don't exist (in case this is the first sensor type)
-- (These are also in common migrations, but we make them idempotent)
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

CREATE TABLE IF NOT EXISTS batteries (
    id BIGSERIAL PRIMARY KEY,
    timestamp TIMESTAMPTZ NOT NULL,
    device_id BIGINT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    battery_percentage INT
);

CREATE INDEX IF NOT EXISTS idx_batteries_timestamp ON batteries(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_batteries_device_id ON batteries(device_id, timestamp DESC);

CREATE TABLE IF NOT EXISTS link_qualities (
    id BIGSERIAL PRIMARY KEY,
    timestamp TIMESTAMPTZ NOT NULL,
    device_id BIGINT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    link_quality INT
);

CREATE INDEX IF NOT EXISTS idx_link_qualities_timestamp ON link_qualities(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_link_qualities_device_id ON link_qualities(device_id, timestamp DESC);

-- Step 2: Check if old schema exists (device_id is VARCHAR)
DO $$
BEGIN
    -- Check if moisture_measurements exists with old schema (device_id as VARCHAR)
    IF EXISTS (
        SELECT 1 
        FROM information_schema.columns 
        WHERE table_name = 'moisture_measurements' 
        AND column_name = 'device_id' 
        AND data_type = 'character varying'
    ) THEN
        -- Old schema detected, perform migration
        
        -- Extract unique devices from moisture_measurements
        INSERT INTO devices (device_id, device_name, short_addr, ieee_addr, created_at, updated_at)
        SELECT DISTINCT 
            device_id,
            device_name,
            short_addr,
            ieee_addr,
            MIN(timestamp) as created_at,
            MAX(timestamp) as updated_at
        FROM moisture_measurements
        WHERE device_id IS NOT NULL
        GROUP BY device_id, device_name, short_addr, ieee_addr
        ON CONFLICT (device_id) DO UPDATE SET
            device_name = COALESCE(EXCLUDED.device_name, devices.device_name),
            short_addr = COALESCE(EXCLUDED.short_addr, devices.short_addr),
            ieee_addr = COALESCE(EXCLUDED.ieee_addr, devices.ieee_addr),
            updated_at = GREATEST(devices.updated_at, EXCLUDED.updated_at);
        
        -- Extract battery data
        INSERT INTO batteries (timestamp, device_id, battery_percentage)
        SELECT 
            m.timestamp,
            d.id,
            m.battery_percentage
        FROM moisture_measurements m
        INNER JOIN devices d ON m.device_id = d.device_id
        WHERE m.battery_percentage IS NOT NULL
        ON CONFLICT DO NOTHING;
        
        -- Extract link quality data
        INSERT INTO link_qualities (timestamp, device_id, link_quality)
        SELECT 
            m.timestamp,
            d.id,
            m.link_quality
        FROM moisture_measurements m
        INNER JOIN devices d ON m.device_id = d.device_id
        WHERE m.link_quality IS NOT NULL
        ON CONFLICT DO NOTHING;
        
        -- Create new normalized moisture_measurements table
        CREATE TABLE moisture_measurements_new (
            id BIGSERIAL PRIMARY KEY,
            timestamp TIMESTAMPTZ NOT NULL,
            device_id BIGINT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
            temperature DECIMAL(5,2),
            humidity DECIMAL(5,2),
            endpoint INT
        );
        
        -- Create indexes
        CREATE INDEX idx_moisture_measurements_new_timestamp ON moisture_measurements_new(timestamp DESC);
        CREATE INDEX idx_moisture_measurements_new_device_id ON moisture_measurements_new(device_id, timestamp DESC);
        
        -- Migrate measurement data
        INSERT INTO moisture_measurements_new (id, timestamp, device_id, temperature, humidity, endpoint)
        SELECT 
            m.id,
            m.timestamp,
            d.id as device_id,
            m.temperature,
            m.humidity,
            m.endpoint
        FROM moisture_measurements m
        INNER JOIN devices d ON m.device_id = d.device_id;
        
        -- Drop old table and rename new one
        DROP TABLE moisture_measurements CASCADE;
        ALTER TABLE moisture_measurements_new RENAME TO moisture_measurements;
        
        -- Rename indexes
        ALTER INDEX idx_moisture_measurements_new_timestamp RENAME TO idx_moisture_measurements_timestamp;
        ALTER INDEX idx_moisture_measurements_new_device_id RENAME TO idx_moisture_measurements_device_id;
        
        -- Update sequence
        SELECT setval('moisture_measurements_id_seq', (SELECT MAX(id) FROM moisture_measurements));
        
    END IF;
END $$;
