-- Rollback: Clear sensor_type for moisture devices
UPDATE devices 
SET sensor_type = NULL 
WHERE sensor_type = 'moisture';
