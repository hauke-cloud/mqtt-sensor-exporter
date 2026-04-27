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

package tasmota

import (
	"context"
	"time"

	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/hauke-cloud/mqtt-sensor-exporter/internal/database"
)

// MessageHandler is the interface for all Tasmota message handlers
type MessageHandler interface {
	HandleMessage(ctx context.Context, msgCtx *MessageContext, payload []byte) error
}

// Dispatcher routes Tasmota MQTT messages to appropriate handlers based on type
type Dispatcher struct {
	client    client.Client
	log       *zap.Logger
	handlers  map[string]MessageHandler
	dbManager *database.Manager
}

// NewDispatcher creates a new Tasmota message dispatcher
// mqtt-sensor-exporter only handles sensor data (telemetry)
func NewDispatcher(c client.Client, log *zap.Logger, dbManager *database.Manager) *Dispatcher {
	d := &Dispatcher{
		client:    c,
		log:       log.With(zap.String("component", "tasmota-dispatcher")),
		handlers:  make(map[string]MessageHandler),
		dbManager: dbManager,
	}

	// Register only telemetry handler for sensor data
	d.RegisterHandler("telemetry", NewTelemetryHandler(c, log.With(zap.String("handler", "telemetry")), dbManager))
	d.RegisterHandler("sensor", NewTelemetryHandler(c, log.With(zap.String("handler", "sensor")), dbManager))

	return d
}

// RegisterHandler registers a custom handler for a message type
func (d *Dispatcher) RegisterHandler(messageType string, handler MessageHandler) {
	d.handlers[messageType] = handler
	d.log.Info("Registered message handler", zap.String("type", messageType))
}

// Dispatch routes a message to the appropriate handler
func (d *Dispatcher) Dispatch(ctx context.Context, messageType, topic, bridgeName, bridgeNamespace string, payload []byte) error {
	handler, ok := d.handlers[messageType]
	if !ok {
		d.log.Warn("No handler registered for message type",
			zap.String("type", messageType),
			zap.String("topic", topic))
		return nil // Not an error, just no handler
	}

	msgCtx := &MessageContext{
		BridgeName:      bridgeName,
		BridgeNamespace: bridgeNamespace,
		Topic:           topic,
		Timestamp:       time.Now(),
	}

	d.log.Debug("Dispatching message",
		zap.String("type", messageType),
		zap.String("topic", topic),
		zap.String("bridge", bridgeName))

	return handler.HandleMessage(ctx, msgCtx, payload)
}

// GetHandler returns a handler for a specific message type
func (d *Dispatcher) GetHandler(messageType string) (MessageHandler, bool) {
	handler, ok := d.handlers[messageType]
	return handler, ok
}
