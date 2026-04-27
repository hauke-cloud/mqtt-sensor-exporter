-- Rollback: Clear sensor_type for valve devices
UPDATE devices 
SET sensor_type = NULL 
WHERE sensor_type = 'valve';
