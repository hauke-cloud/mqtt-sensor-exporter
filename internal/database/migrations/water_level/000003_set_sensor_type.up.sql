-- Update sensor_type for existing water_level devices
UPDATE devices 
SET sensor_type = 'water_level' 
WHERE id IN (
    SELECT DISTINCT device_id 
    FROM water_level_measurements
)
AND sensor_type IS NULL;
