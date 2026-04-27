/*
Copyright 2026 hauke.cloud.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package database

import (
	"embed"
	"fmt"
	"io/fs"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

//go:embed migrations/common/*.sql
var commonMigrationsFS embed.FS

//go:embed migrations/moisture/*.sql
var moistureMigrationsFS embed.FS

//go:embed migrations/valve/*.sql
var valveMigrationsFS embed.FS

//go:embed migrations/water_level/*.sql
var waterLevelMigrationsFS embed.FS

//go:embed migrations/room/*.sql
var roomMigrationsFS embed.FS

// RunMigrationsForSensorTypes runs database migrations for the specified sensor types
// It always runs common migrations first, then sensor-specific migrations
func RunMigrationsForSensorTypes(db *gorm.DB, sensorTypes []string, logger *zap.Logger) error {
	// Always run common migrations first (devices, batteries, link_qualities)
	if err := runMigrationSet(db, "common", commonMigrationsFS, "migrations/common", logger); err != nil {
		return fmt.Errorf("failed to run common migrations: %w", err)
	}

	// Run sensor-specific migrations
	for _, sensorType := range sensorTypes {
		var migrationsFS embed.FS
		var path string

		switch sensorType {
		case "moisture":
			migrationsFS = moistureMigrationsFS
			path = "migrations/moisture"
		case "valve":
			migrationsFS = valveMigrationsFS
			path = "migrations/valve"
		case "water_level":
			migrationsFS = waterLevelMigrationsFS
			path = "migrations/water_level"
		case "room":
			migrationsFS = roomMigrationsFS
			path = "migrations/room"
		default:
			logger.Warn("unknown sensor type, skipping migrations", zap.String("sensorType", sensorType))
			continue
		}

		if err := runMigrationSet(db, sensorType, migrationsFS, path, logger); err != nil {
			return fmt.Errorf("failed to run %s migrations: %w", sensorType, err)
		}
	}

	// Verify critical tables exist
	if err := verifyTablesExist(db, sensorTypes, logger); err != nil {
		logger.Error("database verification failed after migrations", zap.Error(err))
		return fmt.Errorf("database verification failed: %w", err)
	}

	return nil
}

// runMigrationSet runs a specific set of migrations
func runMigrationSet(db *gorm.DB, name string, migrationsFS embed.FS, path string, logger *zap.Logger) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %w", err)
	}

	// Use a unique schema_migrations table per migration set
	tableName := fmt.Sprintf("schema_migrations_%s", name)

	driver, err := postgres.WithInstance(sqlDB, &postgres.Config{
		MigrationsTable: tableName,
	})
	if err != nil {
		return fmt.Errorf("failed to create migration driver: %w", err)
	}

	// Create a sub-filesystem for the specific migration path
	subFS, err := fs.Sub(migrationsFS, path)
	if err != nil {
		return fmt.Errorf("failed to create sub filesystem: %w", err)
	}

	sourceDriver, err := iofs.New(subFS, ".")
	if err != nil {
		return fmt.Errorf("failed to create source driver: %w", err)
	}

	m, err := migrate.NewWithInstance(
		"iofs",
		sourceDriver,
		"postgres",
		driver,
	)
	if err != nil {
		return fmt.Errorf("failed to create migration instance: %w", err)
	}

	// Get current version
	version, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		return fmt.Errorf("failed to get migration version: %w", err)
	}

	if dirty {
		logger.Warn("database is in dirty state, forcing version",
			zap.String("migrationSet", name),
			zap.Uint("version", version))
		if err := m.Force(int(version)); err != nil {
			return fmt.Errorf("failed to force version: %w", err)
		}
	}

	logger.Debug("running migrations",
		zap.String("migrationSet", name),
		zap.Uint("currentVersion", version))

	// Run migrations
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Get new version
	newVersion, _, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		return fmt.Errorf("failed to get new migration version: %w", err)
	}

	if newVersion != version {
		logger.Info("migrations completed",
			zap.String("migrationSet", name),
			zap.Uint("oldVersion", version),
			zap.Uint("newVersion", newVersion))
	} else {
		logger.Debug("migrations up to date",
			zap.String("migrationSet", name),
			zap.Uint("version", version))
	}

	return nil
}

// verifyTablesExist checks that critical tables exist in the database
func verifyTablesExist(db *gorm.DB, sensorTypes []string, logger *zap.Logger) error {
	// Always verify common tables
	tables := []string{"devices", "batteries", "link_qualities"}

	// Add sensor-specific tables
	for _, sensorType := range sensorTypes {
		var tableName string
		switch sensorType {
		case "moisture":
			tableName = "moisture_measurements"
		case "valve":
			tableName = "valve_measurements"
		case "water_level":
			tableName = "water_level_measurements"
		case "room":
			tableName = "room_measurements"
		default:
			continue
		}
		tables = append(tables, tableName)
	}

	for _, tableName := range tables {
		var exists bool
		err := db.Raw(`
			SELECT EXISTS (
				SELECT FROM information_schema.tables 
				WHERE table_schema = 'public' 
				AND table_name = $1
			)
		`, tableName).Scan(&exists).Error

		if err != nil {
			return fmt.Errorf("failed to check if %s table exists: %w", tableName, err)
		}

		if !exists {
			logger.Error("table does not exist despite migrations being marked as complete",
				zap.String("table", tableName),
				zap.String("hint", "database may be in inconsistent state"))
			return fmt.Errorf("%s table does not exist", tableName)
		}

		logger.Debug("table verified", zap.String("table", tableName))
	}

	logger.Info("database verification passed", zap.Strings("tables", tables))
	return nil
}
