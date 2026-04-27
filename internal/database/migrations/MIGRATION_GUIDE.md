# Migration from Denormalized to Normalized Schema

## Overview

Version 000002 migrations handle the transition from the old denormalized schema (where device info, battery, and link quality were stored in each measurement table) to the new normalized schema (where this data is in separate tables).

## What Changed

### Old Schema (Pre-Normalization)
Each measurement table stored everything:
```sql
CREATE TABLE moisture_measurements (
    id BIGSERIAL PRIMARY KEY,
    timestamp TIMESTAMPTZ,
    device_id VARCHAR(255),        -- String device ID
    device_name VARCHAR(255),       -- Duplicated per measurement
    short_addr VARCHAR(50),         -- Duplicated per measurement
    ieee_addr VARCHAR(100),         -- Duplicated per measurement
    temperature DECIMAL(5,2),
    humidity DECIMAL(5,2),
    battery_voltage DECIMAL(4,2),   -- Stored with each measurement
    battery_percentage INT,         -- Stored with each measurement
    link_quality INT,               -- Stored with each measurement
    endpoint INT
);
```

### New Schema (Normalized)
Data separated into logical tables:

**Devices Table (Shared):**
```sql
CREATE TABLE devices (
    id BIGSERIAL PRIMARY KEY,
    device_id VARCHAR(255) UNIQUE,  -- Business key
    device_name VARCHAR(255),
    short_addr VARCHAR(50),
    ieee_addr VARCHAR(100),
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ
);
```

**Batteries Table (Time-series):**
```sql
CREATE TABLE batteries (
    id BIGSERIAL PRIMARY KEY,
    timestamp TIMESTAMPTZ,
    device_id BIGINT REFERENCES devices(id),  -- Foreign key
    battery_percentage INT
);
```

**Link Qualities Table (Time-series):**
```sql
CREATE TABLE link_qualities (
    id BIGSERIAL PRIMARY KEY,
    timestamp TIMESTAMPTZ,
    device_id BIGINT REFERENCES devices(id),  -- Foreign key
    link_quality INT
);
```

**Measurement Tables (Simplified):**
```sql
CREATE TABLE moisture_measurements (
    id BIGSERIAL PRIMARY KEY,
    timestamp TIMESTAMPTZ,
    device_id BIGINT REFERENCES devices(id),  -- Foreign key, not VARCHAR
    temperature DECIMAL(5,2),
    humidity DECIMAL(5,2),
    endpoint INT
    -- No more device_name, short_addr, battery, link_quality here
);
```

## Migration Process

### Automatic Migration (000002_normalize_*_schema.up.sql)

Each sensor type's migration follows these steps:

1. **Check Schema Type**
   - Detects if table has old schema (device_id as VARCHAR)
   - If new schema already exists, does nothing (idempotent)

2. **Create Common Tables** (if needed)
   - Creates devices, batteries, link_qualities tables
   - Uses `IF NOT EXISTS` for safety

3. **Extract Device Data**
   - Finds unique devices from old table
   - Inserts into devices table
   - Handles conflicts (updates if device already exists from another sensor)

4. **Extract Battery Data**
   - Copies all battery measurements to batteries table
   - Links via device_id → devices.id join

5. **Extract Link Quality Data**
   - Copies all link quality measurements to link_qualities table
   - Links via device_id → devices.id join

6. **Create New Measurement Table**
   - Creates *_new table with normalized schema
   - device_id is now BIGINT foreign key

7. **Migrate Measurement Data**
   - Copies measurements with device_id translation
   - Joins old VARCHAR device_id to new BIGINT devices.id

8. **Swap Tables**
   - Drops old table
   - Renames new table
   - Updates indexes and sequences

### Data Preservation

All existing data is preserved:
- ✅ All measurements retained with same ID and timestamp
- ✅ Device information extracted to devices table
- ✅ Battery history extracted to batteries table
- ✅ Link quality history extracted to link_qualities table
- ✅ Foreign key relationships established

### Rollback (000002_normalize_*_schema.down.sql)

If needed, rollback:
1. Detects normalized schema (device_id as BIGINT)
2. Creates old denormalized table
3. Joins data back from devices, batteries, link_qualities
4. Swaps tables back

⚠️ **Warning**: Rollback may lose data if:
- Multiple sensor types share the same database
- Devices table has been updated independently
- Battery/link quality data was added after normalization

## Migration Scenarios

### Scenario 1: Fresh Install
```
supportedSensorTypes: ["moisture"]
```
→ Runs: common/000001 + moisture/000001
→ Result: Clean normalized schema from start

### Scenario 2: Existing Database (Pre-normalization)
```
supportedSensorTypes: ["moisture"]
Database has old moisture_measurements table
```
→ Runs: 
  - common/000001 (creates devices, batteries, link_qualities)
  - moisture/000001 (skipped - table exists)
  - moisture/000002 (detects old schema, migrates data)
→ Result: Data migrated to normalized schema

### Scenario 3: Multiple Sensor Types in Same DB
```
supportedSensorTypes: ["moisture", "room"]
Both have old schema
```
→ Runs:
  - common/000001 (creates shared tables)
  - moisture/000001 (skipped)
  - moisture/000002 (migrates moisture, creates/updates devices)
  - room/000001 (skipped)
  - room/000002 (migrates room, updates existing devices)
→ Result: Shared devices table, both measurements normalized

### Scenario 4: Mixed - One New, One Old
```
supportedSensorTypes: ["moisture", "valve"]
moisture = old schema (VARCHAR device_id)
valve = doesn't exist yet
```
→ Runs:
  - common/000001 (creates shared tables)
  - moisture/000001 (skipped - exists)
  - moisture/000002 (migrates old data)
  - valve/000001 (creates new table - normalized)
  - valve/000002 (nothing to do - already normalized)
→ Result: Both normalized, shared devices

## Verification

After migration, verify:

```sql
-- Check schema
SELECT column_name, data_type 
FROM information_schema.columns 
WHERE table_name = 'moisture_measurements' 
AND column_name = 'device_id';
-- Should return: device_id | bigint

-- Check devices extracted
SELECT COUNT(*) FROM devices;

-- Check measurements preserved
SELECT COUNT(*) FROM moisture_measurements;

-- Check foreign keys work
SELECT m.*, d.device_name, d.short_addr
FROM moisture_measurements m
JOIN devices d ON m.device_id = d.id
LIMIT 5;
```

## Troubleshooting

### Migration Runs But Table Still Old Schema

Check migration tracking:
```sql
SELECT * FROM schema_migrations_moisture;
```

If version shows 2 but schema is old, manually check:
```sql
SELECT column_name, data_type 
FROM information_schema.columns 
WHERE table_name = 'moisture_measurements';
```

### Duplicate Device Entries

If different measurement tables had slightly different device info:
```sql
SELECT device_id, COUNT(*) 
FROM devices 
GROUP BY device_id 
HAVING COUNT(*) > 1;
```

This shouldn't happen due to `ON CONFLICT` clause, but check logs.

### Missing Data After Migration

Check if data was moved to separate tables:
```sql
-- Count devices
SELECT COUNT(*) FROM devices;

-- Count battery measurements
SELECT COUNT(*) FROM batteries;

-- Count link quality measurements  
SELECT COUNT(*) FROM link_qualities;

-- Check relationship
SELECT 
    d.device_id,
    COUNT(m.id) as measurements,
    COUNT(b.id) as battery_readings,
    COUNT(lq.id) as link_quality_readings
FROM devices d
LEFT JOIN moisture_measurements m ON m.device_id = d.id
LEFT JOIN batteries b ON b.device_id = d.id
LEFT JOIN link_qualities lq ON lq.device_id = d.id
GROUP BY d.device_id;
```

## Performance Notes

- Migration runs inside a transaction per sensor type
- Large tables may take time (expect ~1000 rows/second)
- Indexes are created after data migration for speed
- No downtime - new pods will connect after migration completes
- Old pods will fail to insert (schema mismatch) until restart

## Best Practices

1. **Backup first** - Always backup before running migrations
2. **Test on staging** - Run on a copy of production data first
3. **Monitor logs** - Watch for errors during migration
4. **Verify data** - Run verification queries after migration
5. **Keep old pods running** - During migration, they won't write but won't crash
6. **Rolling restart** - After migration, restart pods to use new schema
