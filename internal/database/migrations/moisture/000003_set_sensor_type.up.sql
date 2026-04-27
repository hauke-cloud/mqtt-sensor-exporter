-- Update sensor_type for existing moisture devices
UPDATE devices 
SET sensor_type = 'moisture' 
WHERE id IN (
    SELECT DISTINCT device_id 
    FROM moisture_measurements
)
AND sensor_type IS NULL;
