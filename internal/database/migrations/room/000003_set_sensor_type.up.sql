-- Update sensor_type for existing room devices
UPDATE devices 
SET sensor_type = 'room' 
WHERE id IN (
    SELECT DISTINCT device_id 
    FROM room_measurements
)
AND sensor_type IS NULL;
