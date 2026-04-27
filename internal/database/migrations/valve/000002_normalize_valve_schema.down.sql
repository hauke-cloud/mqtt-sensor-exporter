-- Rollback normalization for valve_measurements
-- WARNING: This will lose the separation between devices, batteries, and link_qualities

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 
        FROM information_schema.columns 
        WHERE table_name = 'valve_measurements' 
        AND column_name = 'device_id' 
        AND data_type = 'bigint'
    ) THEN
        CREATE TABLE valve_measurements_old (
            id BIGSERIAL PRIMARY KEY,
            timestamp TIMESTAMPTZ NOT NULL,
            device_id VARCHAR(255) NOT NULL,
            device_name VARCHAR(255),
            short_addr VARCHAR(50),
            ieee_addr VARCHAR(100),
            power INT,
            last_valve_open_duration INT,
            irrigation_start_time BIGINT,
            irrigation_end_time BIGINT,
            daily_irrigation_volume INT,
            battery_voltage DECIMAL(4,2),
            battery_percentage INT,
            link_quality INT,
            endpoint INT
        );
        
        CREATE INDEX idx_valve_measurements_old_timestamp ON valve_measurements_old(timestamp DESC);
        CREATE INDEX idx_valve_measurements_old_device_id ON valve_measurements_old(device_id);
        
        INSERT INTO valve_measurements_old 
        SELECT 
            v.id,
            v.timestamp,
            d.device_id,
            d.device_name,
            d.short_addr,
            d.ieee_addr,
            v.power,
            v.last_valve_open_duration,
            v.irrigation_start_time,
            v.irrigation_end_time,
            v.daily_irrigation_volume,
            NULL as battery_voltage,
            b.battery_percentage,
            lq.link_quality,
            v.endpoint
        FROM valve_measurements v
        INNER JOIN devices d ON v.device_id = d.id
        LEFT JOIN batteries b ON b.device_id = d.id AND b.timestamp = v.timestamp
        LEFT JOIN link_qualities lq ON lq.device_id = d.id AND lq.timestamp = v.timestamp;
        
        DROP TABLE valve_measurements CASCADE;
        ALTER TABLE valve_measurements_old RENAME TO valve_measurements;
        ALTER INDEX idx_valve_measurements_old_timestamp RENAME TO idx_valve_measurements_timestamp;
        ALTER INDEX idx_valve_measurements_old_device_id RENAME TO idx_valve_measurements_device_id;
    END IF;
END $$;
