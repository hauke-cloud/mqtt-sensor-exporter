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
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"slices"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	iotv1alpha1 "github.com/hauke-cloud/kubernetes-iot-api/api/v1alpha1"
)

// Manager manages database connections and measurement storage
type Manager struct {
	client      client.Client
	log         *zap.Logger
	connections map[string]*Connection
	mu          sync.RWMutex
}

// Connection represents a single database connection with its handlers
type Connection struct {
	database          *iotv1alpha1.Database
	db                *gorm.DB
	moistureHandler   *MoistureHandler
	waterLevelHandler *WaterLevelHandler
	valveHandler      *ValveHandler
	roomHandler       *RoomHandler
	// Future handlers can be added here:
	// powerHandler     *PowerHandler
	// solarHandler     *SolarHandler
}

// NewManager creates a new database manager
func NewManager(c client.Client, log *zap.Logger) *Manager {
	return &Manager{
		client:      c,
		log:         log,
		connections: make(map[string]*Connection),
	}
}

// Connect establishes a connection to a database and initializes handlers
func (m *Manager) Connect(ctx context.Context, database *iotv1alpha1.Database) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("%s/%s", database.Namespace, database.Name)

	m.log.Info("Connecting to database",
		zap.String("database", key),
		zap.String("host", database.Spec.Host),
		zap.String("dbname", database.Spec.Database))

	// Disconnect existing connection if any
	if existing, ok := m.connections[key]; ok {
		if existing.db != nil {
			sqlDB, _ := existing.db.DB()
			if sqlDB != nil {
				_ = sqlDB.Close()
			}
		}
		delete(m.connections, key)
	}

	// Build DSN (Data Source Name)
	dsn, tlsConfig, err := m.buildDSN(ctx, database)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure GORM
	gormConfig := &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	}

	// Create GORM DB instance
	var db *gorm.DB
	if tlsConfig != nil {
		// Parse the DSN and set TLS config
		connConfig, parseErr := pgx.ParseConfig(dsn)
		if parseErr != nil {
			return fmt.Errorf("failed to parse DSN: %w", parseErr)
		}

		// Set the TLS config
		connConfig.TLSConfig = tlsConfig

		// Use stdlib to convert to database/sql
		connStr := stdlib.RegisterConnConfig(connConfig)

		m.log.Debug("Opening database connection with TLS",
			zap.String("database", key),
			zap.String("connStr", connStr))

		db, err = gorm.Open(postgres.New(postgres.Config{
			DriverName: "pgx",
			DSN:        connStr,
		}), gormConfig)
	} else {
		db, err = gorm.Open(postgres.Open(dsn), gormConfig)
	}

	if err != nil {
		m.log.Error("Failed to connect to database",
			zap.String("database", key),
			zap.Error(err))
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	sqlDB, dbErr := db.DB()
	if dbErr != nil {
		return fmt.Errorf("failed to get SQL DB: %w", dbErr)
	}

	if database.Spec.MaxConnections != nil {
		sqlDB.SetMaxOpenConns(int(*database.Spec.MaxConnections))
	}
	if database.Spec.MinConnections != nil {
		sqlDB.SetMaxIdleConns(int(*database.Spec.MinConnections))
	}

	// Initialize connection
	conn := &Connection{
		database: database,
		db:       db,
	}

	// Initialize handlers based on supported sensor types
	for _, sensorType := range database.Spec.SupportedSensorTypes {
		switch sensorType {
		case "moisture":
			handler, err := NewMoistureHandler(db, m.log.With(zap.String("handler", "moisture")))
			if err != nil {
				return fmt.Errorf("failed to connect to database: %w", err)
			}
			conn.moistureHandler = handler
			m.log.Info("Moisture handler initialized", zap.String("database", key))

		case "water_level":
			handler := NewWaterLevelHandler(db, m.log.With(zap.String("handler", "water_level")))
			if err := handler.Initialize(ctx); err != nil {
				return fmt.Errorf("failed to initialize water level handler: %w", err)
			}
			conn.waterLevelHandler = handler
			m.log.Info("Water level handler initialized", zap.String("database", key))

		case "valve":
			handler := NewValveHandler(db, m.log.With(zap.String("handler", "valve")))
			if err := handler.Initialize(ctx); err != nil {
				return fmt.Errorf("failed to initialize valve handler: %w", err)
			}
			conn.valveHandler = handler
			m.log.Info("Valve handler initialized", zap.String("database", key))

		case "room":
			handler := NewRoomHandler(db, m.log.With(zap.String("handler", "room")))
			if err := handler.Initialize(ctx); err != nil {
				return fmt.Errorf("failed to initialize room handler: %w", err)
			}
			conn.roomHandler = handler
			m.log.Info("Room handler initialized", zap.String("database", key))

			// Future sensor types:
			// case "power":
			//     handler := NewPowerHandler(db, m.log.With(zap.String("handler", "power")))
			//     if err := handler.Initialize(ctx); err != nil {
			//         return fmt.Errorf("failed to initialize power handler: %w", err)
			//     }
			//     conn.powerHandler = handler
		}
	}

	m.connections[key] = conn

	m.log.Info("Successfully connected to database",
		zap.String("database", key),
		zap.String("host", database.Spec.Host))

	return nil
}

// Disconnect closes the connection to a database
func (m *Manager) Disconnect(namespace, name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("%s/%s", namespace, name)
	if conn, ok := m.connections[key]; ok {
		if conn.db != nil {
			sqlDB, _ := conn.db.DB()
			if sqlDB != nil {
				_ = sqlDB.Close()
			}
		}
		delete(m.connections, key)
		m.log.Info("Disconnected from database", zap.String("database", key))
	}
}

// StoreMeasurement stores a measurement for a device based on its sensor type
func (m *Manager) StoreMeasurement(ctx context.Context, deviceID, sensorType string, payload map[string]any) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.log.Debug("StoreMeasurement called",
		zap.String("deviceID", deviceID),
		zap.String("sensorType", sensorType),
		zap.Int("payloadSize", len(payload)),
		zap.Int("numConnections", len(m.connections)))

	// Find database that supports this sensor type
	var targetConn *Connection
	for key, conn := range m.connections {
		m.log.Debug("Checking database connection",
			zap.String("database", key),
			zap.Strings("supportedTypes", conn.database.Spec.SupportedSensorTypes))

		if slices.Contains(conn.database.Spec.SupportedSensorTypes, sensorType) {
			targetConn = conn
			m.log.Debug("Found matching database",
				zap.String("database", key),
				zap.String("sensorType", sensorType))
			break
		}
	}

	if targetConn == nil {
		m.log.Warn("No database found for sensor type",
			zap.String("sensorType", sensorType),
			zap.String("deviceID", deviceID))
		return fmt.Errorf("no database found for sensor type: %s", sensorType)
	}

	// Route to appropriate handler
	switch sensorType {
	case "moisture":
		if targetConn.moistureHandler == nil {
			return fmt.Errorf("moisture handler not initialized")
		}
		return targetConn.moistureHandler.StoreMeasurement(ctx, deviceID, payload)

	case "water_level":
		if targetConn.waterLevelHandler == nil {
			return fmt.Errorf("water level handler not initialized")
		}
		return targetConn.waterLevelHandler.StoreMeasurement(ctx, deviceID, payload)

	case "valve":
		if targetConn.valveHandler == nil {
			return fmt.Errorf("valve handler not initialized")
		}
		return targetConn.valveHandler.StoreMeasurement(ctx, deviceID, payload)

	case "room":
		if targetConn.roomHandler == nil {
			return fmt.Errorf("room handler not initialized")
		}
		return targetConn.roomHandler.StoreMeasurement(ctx, deviceID, payload)

	// Future sensor types:
	// case "power":
	//     if targetConn.powerHandler == nil {
	//         return fmt.Errorf("power handler not initialized")
	//     }
	//     return targetConn.powerHandler.StoreMeasurement(ctx, deviceID, payload)

	default:
		return fmt.Errorf("unsupported sensor type: %s", sensorType)
	}
}

// buildDSN constructs the PostgreSQL connection string
func (m *Manager) buildDSN(ctx context.Context, database *iotv1alpha1.Database) (string, *tls.Config, error) {
	dsn := fmt.Sprintf("host=%s port=%d dbname=%s user=%s",
		database.Spec.Host,
		database.Spec.Port,
		database.Spec.Database,
		database.Spec.Username)

	// Get password from secret if specified
	if database.Spec.PasswordSecretRef != nil {
		password, err := m.getSecretValue(ctx, database.Namespace, database.Spec.PasswordSecretRef.Name, "password")
		if err != nil {
			return "", nil, fmt.Errorf("failed to connect to database: %w", err)
		}
		dsn += fmt.Sprintf(" password=%s", password)
	}

	// SSL mode
	sslMode := "require"
	if database.Spec.SSLMode != "" {
		sslMode = database.Spec.SSLMode
	}
	dsn += fmt.Sprintf(" sslmode=%s", sslMode)

	// TLS configuration
	var tlsConfig *tls.Config

	// CA certificate for verification
	if database.Spec.CASecretRef != nil {
		caCert, err := m.getSecretValue(ctx, database.Namespace, database.Spec.CASecretRef.Name, "ca.crt")
		if err != nil {
			return "", nil, fmt.Errorf("failed to connect to database: %w", err)
		}

		m.log.Debug("Loading CA certificate from secret",
			zap.String("secret", database.Spec.CASecretRef.Name),
			zap.Int("certLength", len(caCert)))

		if tlsConfig == nil {
			tlsConfig = &tls.Config{}
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM([]byte(caCert)) {
			m.log.Error("Failed to parse CA certificate - invalid PEM format",
				zap.String("secret", database.Spec.CASecretRef.Name))
			return "", nil, fmt.Errorf("failed to parse CA certificate from secret %s: invalid PEM format", database.Spec.CASecretRef.Name)
		}
		tlsConfig.RootCAs = caCertPool

		// For verify-full, also verify the server hostname
		if sslMode == "verify-full" {
			tlsConfig.ServerName = database.Spec.Host
		}

		m.log.Info("CA certificate loaded successfully",
			zap.String("database", database.Name),
			zap.String("secret", database.Spec.CASecretRef.Name),
			zap.String("sslMode", sslMode),
			zap.String("serverName", database.Spec.Host))
	} else if sslMode == "require" || sslMode == "prefer" {
		// If SSL is required but no CA cert provided, skip verification
		// This is common for internal/self-signed certificates
		if tlsConfig == nil {
			tlsConfig = &tls.Config{}
		}
		tlsConfig.InsecureSkipVerify = true
		m.log.Warn("SSL enabled without CA certificate, skipping certificate verification",
			zap.String("database", database.Name),
			zap.String("sslMode", sslMode))
	}

	// Client certificate for mutual TLS
	if database.Spec.ClientCertSecretRef != nil {
		certPEM, err := m.getSecretValue(ctx, database.Namespace, database.Spec.ClientCertSecretRef.Name, "tls.crt")
		if err != nil {
			return "", nil, fmt.Errorf("failed to connect to database: %w", err)
		}

		keyPEM, err := m.getSecretValue(ctx, database.Namespace, database.Spec.ClientCertSecretRef.Name, "tls.key")
		if err != nil {
			return "", nil, fmt.Errorf("failed to connect to database: %w", err)
		}

		cert, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
		if err != nil {
			return "", nil, fmt.Errorf("failed to connect to database: %w", err)
		}

		if tlsConfig == nil {
			tlsConfig = &tls.Config{}
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return dsn, tlsConfig, nil
}

// getSecretValue retrieves a value from a Kubernetes Secret
func (m *Manager) getSecretValue(ctx context.Context, namespace, secretName, key string) (string, error) {
	secret := &corev1.Secret{}
	err := m.client.Get(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      secretName,
	}, secret)

	if err != nil {
		return "", err
	}

	value, ok := secret.Data[key]
	if !ok {
		return "", fmt.Errorf("key %s not found in secret %s", key, secretName)
	}

	return string(value), nil
}
