-- Remove sensor_type column from devices table
DROP INDEX IF EXISTS idx_devices_sensor_type;
ALTER TABLE devices DROP COLUMN IF EXISTS sensor_type;
