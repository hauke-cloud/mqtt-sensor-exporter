-- Add sensor_type column to devices table
ALTER TABLE devices ADD COLUMN IF NOT EXISTS sensor_type VARCHAR(50);

-- Create index for sensor_type
CREATE INDEX IF NOT EXISTS idx_devices_sensor_type ON devices(sensor_type);
