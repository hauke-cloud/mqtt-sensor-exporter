-- Update sensor_type for existing valve devices
UPDATE devices 
SET sensor_type = 'valve' 
WHERE id IN (
    SELECT DISTINCT device_id 
    FROM valve_measurements
)
AND sensor_type IS NULL;
