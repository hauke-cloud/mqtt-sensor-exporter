-- Rollback normalization (restore denormalized schema)
-- WARNING: This will lose the separation between devices, batteries, and link_qualities

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 
        FROM information_schema.columns 
        WHERE table_name = 'moisture_measurements' 
        AND column_name = 'device_id' 
        AND data_type = 'bigint'
    ) THEN
        -- Create old denormalized table
        CREATE TABLE moisture_measurements_old (
            id BIGSERIAL PRIMARY KEY,
            timestamp TIMESTAMPTZ NOT NULL,
            device_id VARCHAR(255) NOT NULL,
            device_name VARCHAR(255),
            short_addr VARCHAR(50),
            ieee_addr VARCHAR(100),
            temperature DECIMAL(5,2),
            humidity DECIMAL(5,2),
            battery_voltage DECIMAL(4,2),
            battery_percentage INT,
            link_quality INT,
            endpoint INT
        );
        
        CREATE INDEX idx_moisture_measurements_old_timestamp ON moisture_measurements_old(timestamp DESC);
        CREATE INDEX idx_moisture_measurements_old_device_id ON moisture_measurements_old(device_id);
        
        -- Restore data from normalized tables
        INSERT INTO moisture_measurements_old 
        SELECT 
            m.id,
            m.timestamp,
            d.device_id,
            d.device_name,
            d.short_addr,
            d.ieee_addr,
            m.temperature,
            m.humidity,
            NULL as battery_voltage,
            b.battery_percentage,
            lq.link_quality,
            m.endpoint
        FROM moisture_measurements m
        INNER JOIN devices d ON m.device_id = d.id
        LEFT JOIN batteries b ON b.device_id = d.id AND b.timestamp = m.timestamp
        LEFT JOIN link_qualities lq ON lq.device_id = d.id AND lq.timestamp = m.timestamp;
        
        DROP TABLE moisture_measurements CASCADE;
        ALTER TABLE moisture_measurements_old RENAME TO moisture_measurements;
        
        ALTER INDEX idx_moisture_measurements_old_timestamp RENAME TO idx_moisture_measurements_timestamp;
        ALTER INDEX idx_moisture_measurements_old_device_id RENAME TO idx_moisture_measurements_device_id;
    END IF;
END $$;
