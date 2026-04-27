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
	databaseiotgorm "github.com/hauke-cloud/database-iot-gorm"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// RunMigrationsForSensorTypes runs database migrations for the specified sensor types
// This is a wrapper that delegates to the database-iot-gorm package
func RunMigrationsForSensorTypes(db *gorm.DB, sensorTypes []string, logger *zap.Logger) error {
	return databaseiotgorm.RunMigrationsForSensorTypes(db, sensorTypes, logger)
}
