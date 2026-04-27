-- Rollback: Clear sensor_type for water_level devices
UPDATE devices 
SET sensor_type = NULL 
WHERE sensor_type = 'water_level';
