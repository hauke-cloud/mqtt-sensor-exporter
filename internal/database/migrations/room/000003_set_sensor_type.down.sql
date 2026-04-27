-- Rollback: Clear sensor_type for room devices
UPDATE devices 
SET sensor_type = NULL 
WHERE sensor_type = 'room';
