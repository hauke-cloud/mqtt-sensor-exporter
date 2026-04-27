# Database Migrations

This directory contains SQL migration files for the mqtt-sensor-exporter database schema, organized by sensor type.

## Overview

The mqtt-sensor-exporter uses [golang-migrate](https://github.com/golang-migrate/migrate) to manage database schema migrations. Migrations are embedded into the binary and run automatically when connecting to a Database CR discovered via the Kubernetes IoT API.

## Migration Structure

Migrations are organized into subdirectories based on their scope:

- **`common/`** - Shared tables required by all sensor types (devices, batteries, link_qualities)
- **`moisture/`** - Moisture sensor specific tables (moisture_measurements)
- **`valve/`** - Valve sensor specific tables (valve_measurements)
- **`water_level/`** - Water level sensor specific tables (water_level_measurements)
- **`room/`** - Room sensor specific tables (room_measurements)

## Migration Flow

1. **Database Discovery**: The mqtt-sensor-exporter watches for `databases.iot.hauke.cloud` Custom Resources in the Kubernetes cluster
2. **Connection**: When a Database CR with matching `supportedSensorTypes` is found, the exporter connects to it
3. **Common Migration**: The `common` migrations are always run first to create shared tables
4. **Sensor-Specific Migrations**: Only migrations for the `supportedSensorTypes` specified in the Database CR are run
5. **Verification**: After migration, the system verifies all expected tables exist
6. **Handler Initialization**: Sensor-specific handlers are initialized

### Example: Multiple Databases

Database A (Moisture + Room sensors):
```yaml
spec:
  supportedSensorTypes:
    - moisture
    - room
```
→ Runs: `common` + `moisture` + `room` migrations

Database B (Valve sensors only):
```yaml
spec:
  supportedSensorTypes:
    - valve
```
→ Runs: `common` + `valve` migrations

Both databases get the shared tables (devices, batteries, link_qualities) but only the measurement tables they need.

## Migration Files

Each migration set uses its own tracking table:
- `schema_migrations_common` - Tracks common migrations
- `schema_migrations_moisture` - Tracks moisture migrations
- `schema_migrations_valve` - Tracks valve migrations
- `schema_migrations_water_level` - Tracks water level migrations
- `schema_migrations_room` - Tracks room migrations

Migrations follow the naming convention: `{version}_{description}.{up|down}.sql`

- **up.sql**: Applied when migrating forward
- **down.sql**: Applied when rolling back (rarely used in production)

### Current Migrations

#### Common (000001_create_common_tables)

Creates shared tables used across all sensor types:

**Tables:**
- `devices` - Stores unique device information (DeviceID, DeviceName, ShortAddr, IEEEAddr)
- `batteries` - Battery percentage measurements over time
- `link_qualities` - Link quality measurements over time

#### Sensor-Specific (000001_create_*_measurements)

Each sensor type gets its own measurement table with appropriate fields:

- **moisture_measurements** - Temperature, humidity, endpoint
- **valve_measurements** - Power, duration, irrigation times, volume, endpoint
- **water_level_measurements** - Level, endpoint
- **room_measurements** - Temperature, humidity, endpoint

All measurement tables reference the `devices` table via foreign key relationships with CASCADE delete constraints.

## Adding New Migrations

### Adding to an existing sensor type:

1. Create files in the appropriate subdirectory:
   ```
   migrations/moisture/000002_add_soil_ph_column.up.sql
   migrations/moisture/000002_add_soil_ph_column.down.sql
   ```

2. The version number (000002) must be sequential within that subdirectory

3. Write your SQL statements

### Adding a new sensor type:

1. Create a new subdirectory: `migrations/newsensor/`

2. Create initial migration files:
   ```
   migrations/newsensor/000001_create_newsensor_measurements.up.sql
   migrations/newsensor/000001_create_newsensor_measurements.down.sql
   ```

3. Update `migrations.go`:
   - Add `//go:embed migrations/newsensor/*.sql`
   - Add case in `RunMigrationsForSensorTypes` switch
   - Add table verification in `verifyTablesExist`

4. Update `manager.go` to handle the new sensor type

## Migration Verification

After migrations run, the system automatically verifies that expected tables exist:
- Common tables: `devices`, `batteries`, `link_qualities`
- Sensor-specific tables based on `supportedSensorTypes`

If verification fails, the connection will not be established and an error will be logged.

## Troubleshooting

### Dirty Migration State

If a migration fails mid-execution, the database may be in a "dirty" state. The exporter will log:
```
database is in dirty state, forcing version
```

And automatically attempt to force the version. If this doesn't resolve the issue:

1. Check the appropriate `schema_migrations_*` table in your database
2. Manually fix any incomplete changes
3. Update or delete the row in the tracking table as needed

### Migration Fails

If migrations fail consistently:

1. Check database logs for detailed error messages
2. Verify database user has sufficient privileges (CREATE TABLE, CREATE INDEX, etc.)
3. Check for naming conflicts with existing tables
4. Review the migration SQL for syntax errors
5. Ensure the Database CR's `supportedSensorTypes` are correct

### Common Tables Already Exist

If you connect multiple sensor types to the same database at different times, the common migrations will be idempotent (use `IF NOT EXISTS`). This is expected and safe.

## Best Practices

1. **Use IF NOT EXISTS** for tables and indexes to ensure idempotency
2. **Test migrations** on a development database first
3. **Keep migrations small** - one logical change per migration
4. **Document complex changes** with SQL comments
5. **Consider data migration** when modifying existing tables
6. **Separate concerns** - keep sensor-specific tables in their own migration sets

