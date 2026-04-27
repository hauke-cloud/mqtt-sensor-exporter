-- Rollback normalization for water_level_measurements
-- WARNING: This will lose the separation between devices, batteries, and link_qualities

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 
        FROM information_schema.columns 
        WHERE table_name = 'water_level_measurements' 
        AND column_name = 'device_id' 
        AND data_type = 'bigint'
    ) THEN
        CREATE TABLE water_level_measurements_old (
            id BIGSERIAL PRIMARY KEY,
            timestamp TIMESTAMPTZ NOT NULL,
            device_id VARCHAR(255) NOT NULL,
            device_name VARCHAR(255),
            short_addr VARCHAR(50),
            ieee_addr VARCHAR(100),
            level INT,
            battery_percentage INT,
            link_quality INT,
            endpoint INT
        );
        
        CREATE INDEX idx_water_level_measurements_old_timestamp ON water_level_measurements_old(timestamp DESC);
        CREATE INDEX idx_water_level_measurements_old_device_id ON water_level_measurements_old(device_id);
        
        INSERT INTO water_level_measurements_old 
        SELECT 
            w.id,
            w.timestamp,
            d.device_id,
            d.device_name,
            d.short_addr,
            d.ieee_addr,
            w.level,
            b.battery_percentage,
            lq.link_quality,
            w.endpoint
        FROM water_level_measurements w
        INNER JOIN devices d ON w.device_id = d.id
        LEFT JOIN batteries b ON b.device_id = d.id AND b.timestamp = w.timestamp
        LEFT JOIN link_qualities lq ON lq.device_id = d.id AND lq.timestamp = w.timestamp;
        
        DROP TABLE water_level_measurements CASCADE;
        ALTER TABLE water_level_measurements_old RENAME TO water_level_measurements;
        ALTER INDEX idx_water_level_measurements_old_timestamp RENAME TO idx_water_level_measurements_timestamp;
        ALTER INDEX idx_water_level_measurements_old_device_id RENAME TO idx_water_level_measurements_device_id;
    END IF;
END $$;
