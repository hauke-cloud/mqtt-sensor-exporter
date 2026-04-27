-- Rollback normalization for room_measurements
-- WARNING: This will lose the separation between devices and link_qualities

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 
        FROM information_schema.columns 
        WHERE table_name = 'room_measurements' 
        AND column_name = 'device_id' 
        AND data_type = 'bigint'
    ) THEN
        CREATE TABLE room_measurements_old (
            id BIGSERIAL PRIMARY KEY,
            timestamp TIMESTAMPTZ NOT NULL,
            device_id VARCHAR(255) NOT NULL,
            device_name VARCHAR(255),
            short_addr VARCHAR(50),
            ieee_addr VARCHAR(100),
            temperature DECIMAL(5,2),
            humidity DECIMAL(5,2),
            link_quality INT,
            endpoint INT
        );
        
        CREATE INDEX idx_room_measurements_old_timestamp ON room_measurements_old(timestamp DESC);
        CREATE INDEX idx_room_measurements_old_device_id ON room_measurements_old(device_id);
        
        INSERT INTO room_measurements_old 
        SELECT 
            r.id,
            r.timestamp,
            d.device_id,
            d.device_name,
            d.short_addr,
            d.ieee_addr,
            r.temperature,
            r.humidity,
            lq.link_quality,
            r.endpoint
        FROM room_measurements r
        INNER JOIN devices d ON r.device_id = d.id
        LEFT JOIN link_qualities lq ON lq.device_id = d.id AND lq.timestamp = r.timestamp;
        
        DROP TABLE room_measurements CASCADE;
        ALTER TABLE room_measurements_old RENAME TO room_measurements;
        ALTER INDEX idx_room_measurements_old_timestamp RENAME TO idx_room_measurements_timestamp;
        ALTER INDEX idx_room_measurements_old_device_id RENAME TO idx_room_measurements_device_id;
    END IF;
END $$;
