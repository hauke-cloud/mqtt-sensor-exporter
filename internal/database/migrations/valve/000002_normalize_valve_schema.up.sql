-- Migration: Normalize valve_measurements table
-- This migration extracts device, battery, and link quality data into separate tables

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

-- Step 2: Check if old schema exists
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 
        FROM information_schema.columns 
        WHERE table_name = 'valve_measurements' 
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
        FROM valve_measurements
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
            v.timestamp,
            d.id,
            v.battery_percentage
        FROM valve_measurements v
        INNER JOIN devices d ON v.device_id = d.device_id
        WHERE v.battery_percentage IS NOT NULL
        ON CONFLICT DO NOTHING;
        
        -- Extract link quality data
        INSERT INTO link_qualities (timestamp, device_id, link_quality)
        SELECT 
            v.timestamp,
            d.id,
            v.link_quality
        FROM valve_measurements v
        INNER JOIN devices d ON v.device_id = d.device_id
        WHERE v.link_quality IS NOT NULL
        ON CONFLICT DO NOTHING;
        
        -- Create new table
        CREATE TABLE valve_measurements_new (
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
        
        CREATE INDEX idx_valve_measurements_new_timestamp ON valve_measurements_new(timestamp DESC);
        CREATE INDEX idx_valve_measurements_new_device_id ON valve_measurements_new(device_id, timestamp DESC);
        
        -- Migrate data
        INSERT INTO valve_measurements_new (id, timestamp, device_id, power, last_valve_open_duration, 
                                            irrigation_start_time, irrigation_end_time, daily_irrigation_volume, endpoint)
        SELECT 
            v.id,
            v.timestamp,
            d.id as device_id,
            v.power,
            v.last_valve_open_duration,
            v.irrigation_start_time,
            v.irrigation_end_time,
            v.daily_irrigation_volume,
            v.endpoint
        FROM valve_measurements v
        INNER JOIN devices d ON v.device_id = d.device_id;
        
        DROP TABLE valve_measurements CASCADE;
        ALTER TABLE valve_measurements_new RENAME TO valve_measurements;
        ALTER INDEX idx_valve_measurements_new_timestamp RENAME TO idx_valve_measurements_timestamp;
        ALTER INDEX idx_valve_measurements_new_device_id RENAME TO idx_valve_measurements_device_id;
        
        SELECT setval('valve_measurements_id_seq', (SELECT MAX(id) FROM valve_measurements));
    END IF;
END $$;
