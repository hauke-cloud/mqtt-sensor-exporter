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

package mqtt

import (
	"context"
	"crypto/tls"
	"fmt"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	iotv1alpha1 "github.com/hauke-cloud/kubernetes-iot-api/api/v1alpha1"
	"github.com/hauke-cloud/mqtt-sensor-exporter/internal/database"
	"github.com/hauke-cloud/mqtt-sensor-exporter/internal/tasmota"
)

const (
	deviceTypeTasmota     = "tasmota"
	deviceTypeZigbee2MQTT = "zigbee2mqtt"
)

// BridgeManager manages MQTT connections for multiple bridges
type BridgeManager struct {
	client            client.Client
	log               *zap.Logger
	bridges           map[string]*BridgeConnection
	mu                sync.RWMutex
	tasmotaDispatcher *tasmota.Dispatcher
	dbManager         *database.Manager
}

// BridgeConnection represents a single MQTT bridge connection
type BridgeConnection struct {
	bridge     *iotv1alpha1.MQTTBridge
	mqttClient mqtt.Client
	connected  bool
	lastSeen   time.Time
	mu         sync.RWMutex
}

// NewBridgeManager creates a new MQTT bridge manager
func NewBridgeManager(c client.Client, log *zap.Logger, dbManager *database.Manager) *BridgeManager {
	m := &BridgeManager{
		client:    c,
		log:       log,
		bridges:   make(map[string]*BridgeConnection),
		dbManager: dbManager,
	}
	// Create dispatcher with database manager
	m.tasmotaDispatcher = tasmota.NewDispatcher(c, log.With(zap.String("component", "tasmota")), dbManager)
	return m
}

// Connect establishes connection to an MQTT bridge
func (m *BridgeManager) Connect(ctx context.Context, bridge *iotv1alpha1.MQTTBridge) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("%s/%s", bridge.Namespace, bridge.Name)

	m.log.Info("Connecting to MQTT bridge",
		zap.String("bridge", key),
		zap.String("host", bridge.Spec.Host),
		zap.Int32("port", bridge.Spec.Port))

	// Disconnect existing connection if any
	if existing, ok := m.bridges[key]; ok {
		if existing.mqttClient != nil && existing.mqttClient.IsConnected() {
			m.log.Info("Disconnecting existing connection", zap.String("bridge", key))
			existing.mqttClient.Disconnect(250)
		}
	}

	// Get credentials from secret if specified
	username := ""
	password := ""
	if bridge.Spec.CredentialsSecretRef != nil {
		m.log.Debug("Fetching credentials from secret",
			zap.String("secret", bridge.Spec.CredentialsSecretRef.Name))
		var err error
		username, password, err = m.getCredentials(ctx, bridge)
		if err != nil {
			m.log.Error("Failed to get credentials", zap.Error(err))
			return fmt.Errorf("failed to get credentials: %w", err)
		}
		m.log.Debug("Credentials loaded successfully")
	}

	// Configure MQTT client
	opts := mqtt.NewClientOptions()
	brokerURL := fmt.Sprintf("tcp://%s:%d", bridge.Spec.Host, bridge.Spec.Port)

	if bridge.Spec.TLS != nil && bridge.Spec.TLS.Enabled {
		brokerURL = fmt.Sprintf("ssl://%s:%d", bridge.Spec.Host, bridge.Spec.Port)
		tlsConfig := &tls.Config{
			InsecureSkipVerify: bridge.Spec.TLS.InsecureSkipVerify,
		}
		opts.SetTLSConfig(tlsConfig)
	}

	opts.AddBroker(brokerURL)

	if username != "" {
		opts.SetUsername(username)
		opts.SetPassword(password)
	}

	clientID := bridge.Spec.ClientID
	if clientID == "" {
		clientID = fmt.Sprintf("mqtt-sensor-exporter-%s-%s", bridge.Namespace, bridge.Name)
	}
	opts.SetClientID(clientID)

	m.log.Debug("MQTT broker configuration",
		zap.String("broker", brokerURL),
		zap.String("clientID", clientID),
		zap.Bool("tls", bridge.Spec.TLS != nil && bridge.Spec.TLS.Enabled),
		zap.Bool("hasCredentials", username != ""))

	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(false) // Don't retry on initial connection - fail fast
	opts.SetConnectRetryInterval(10 * time.Second)
	opts.SetConnectTimeout(5 * time.Second) // Set connection timeout to avoid hanging in tests

	bridgeConn := &BridgeConnection{
		bridge: bridge,
	}

	// Set connection handlers
	opts.SetOnConnectHandler(func(c mqtt.Client) {
		m.onConnect(ctx, bridgeConn)
	})
	opts.SetConnectionLostHandler(func(c mqtt.Client, err error) {
		m.onConnectionLost(bridgeConn, err)
	})

	// Create and connect client
	m.log.Info("Attempting MQTT connection", zap.String("broker", brokerURL))
	mqttClient := mqtt.NewClient(opts)
	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		m.log.Error("MQTT connection failed",
			zap.String("broker", brokerURL),
			zap.Error(token.Error()))
		return fmt.Errorf("failed to connect to MQTT broker: %w", token.Error())
	}

	bridgeConn.mqttClient = mqttClient
	bridgeConn.connected = true
	bridgeConn.lastSeen = time.Now()
	m.bridges[key] = bridgeConn

	m.log.Info("Successfully connected to MQTT bridge",
		zap.String("bridge", key),
		zap.String("broker", brokerURL))
	return nil
}

// Disconnect closes the connection to an MQTT bridge
func (m *BridgeManager) Disconnect(namespace, name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("%s/%s", namespace, name)
	if conn, ok := m.bridges[key]; ok {
		if conn.mqttClient != nil && conn.mqttClient.IsConnected() {
			conn.mqttClient.Disconnect(250)
		}
		delete(m.bridges, key)
		m.log.Info("Disconnected from MQTT bridge", zap.String("bridge", key))
	}
}

// IsConnected checks if a bridge is connected
func (m *BridgeManager) IsConnected(namespace, name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := fmt.Sprintf("%s/%s", namespace, name)
	if conn, ok := m.bridges[key]; ok {
		return conn.connected && conn.mqttClient != nil && conn.mqttClient.IsConnected()
	}
	return false
}

// getCredentials retrieves MQTT credentials from a Kubernetes Secret
func (m *BridgeManager) getCredentials(ctx context.Context, bridge *iotv1alpha1.MQTTBridge) (string, string, error) {
	secretRef := bridge.Spec.CredentialsSecretRef
	namespace := secretRef.Namespace
	if namespace == "" {
		namespace = bridge.Namespace
	}

	secret := &corev1.Secret{}
	if err := m.client.Get(ctx, types.NamespacedName{
		Name:      secretRef.Name,
		Namespace: namespace,
	}, secret); err != nil {
		return "", "", err
	}

	username := string(secret.Data["username"])
	password := string(secret.Data["password"])

	return username, password, nil
}

// onConnect is called when connection to MQTT broker is established
func (m *BridgeManager) onConnect(ctx context.Context, conn *BridgeConnection) {
	conn.mu.Lock()
	conn.connected = true
	conn.lastSeen = time.Now()
	conn.mu.Unlock()

	m.log.Info("MQTT connection established", zap.String("bridge", conn.bridge.Name))

	// Subscribe to all configured topics
	m.subscribeToTopics(ctx, conn)
}

// onConnectionLost is called when connection to MQTT broker is lost
func (m *BridgeManager) onConnectionLost(conn *BridgeConnection, err error) {
	conn.mu.Lock()
	conn.connected = false
	conn.mu.Unlock()

	m.log.Error("MQTT connection lost",
		zap.String("bridge", conn.bridge.Name),
		zap.Error(err))
}

// subscribeToTopics subscribes to all configured topics for a bridge
func (m *BridgeManager) subscribeToTopics(ctx context.Context, conn *BridgeConnection) {
	// mqtt-sensor-exporter only handles sensor data topics
	sensorTopicTypes := map[string]bool{
		"telemetry": true,
		"sensor":    true,
	}

	if len(conn.bridge.Spec.Topics) > 0 {
		m.log.Info("Subscribing to topics",
			zap.String("bridge", conn.bridge.Name),
			zap.Int("topicCount", len(conn.bridge.Spec.Topics)))

		for _, topicSub := range conn.bridge.Spec.Topics {
			// Only subscribe to sensor data topics
			if sensorTopicTypes[topicSub.Type] {
				m.subscribeToTopic(ctx, conn, &topicSub)
			} else {
				m.log.Debug("Skipping non-sensor topic type",
					zap.String("topic", topicSub.Topic),
					zap.String("type", topicSub.Type))
			}
		}
		return
	}
}

// subscribeToTopic subscribes to a single topic
func (m *BridgeManager) subscribeToTopic(ctx context.Context, conn *BridgeConnection, topicSub *iotv1alpha1.TopicSubscription) {
	// Check if client is still connected
	if conn.mqttClient == nil || !conn.mqttClient.IsConnected() {
		m.log.Debug("MQTT client not connected, skipping subscription",
			zap.String("topic", topicSub.Topic))
		return
	}

	qos := byte(0)
	if topicSub.QoS != nil {
		qos = byte(*topicSub.QoS)
	}

	handler := func(mqttClient mqtt.Client, msg mqtt.Message) {
		m.handleMessage(ctx, conn, topicSub, msg)
	}

	if token := conn.mqttClient.Subscribe(topicSub.Topic, qos, handler); token.Wait() && token.Error() != nil {
		m.log.Error("Failed to subscribe to topic",
			zap.String("topic", topicSub.Topic),
			zap.String("type", topicSub.Type),
			zap.Error(token.Error()))
	} else {
		m.log.Info("Subscribed to topic",
			zap.String("topic", topicSub.Topic),
			zap.String("type", topicSub.Type),
			zap.Int("qos", int(qos)))
	}
}

// handleMessage processes an incoming MQTT message
func (m *BridgeManager) handleMessage(ctx context.Context, conn *BridgeConnection, topicSub *iotv1alpha1.TopicSubscription, msg mqtt.Message) {
	m.log.Info("Received message",
		zap.String("topic", msg.Topic()),
		zap.String("type", topicSub.Type),
		zap.String("bridge", conn.bridge.Name),
		zap.Int("payloadSize", len(msg.Payload())))

	// Route to appropriate handler based on device type
	switch conn.bridge.Spec.DeviceType {
	case deviceTypeTasmota:
		// Dispatch to Tasmota handler
		if err := m.tasmotaDispatcher.Dispatch(
			ctx,
			topicSub.Type,
			msg.Topic(),
			conn.bridge.Name,
			conn.bridge.Namespace,
			msg.Payload(),
		); err != nil {
			m.log.Error("Failed to dispatch Tasmota message",
				zap.String("topic", msg.Topic()),
				zap.String("type", topicSub.Type),
				zap.Error(err))
		}
	case deviceTypeZigbee2MQTT:
		// TODO: Implement Zigbee2MQTT handler
		m.log.Debug("Zigbee2MQTT message handling not yet implemented",
			zap.String("topic", msg.Topic()))
	default:
		m.log.Debug("Generic message handling not yet implemented",
			zap.String("topic", msg.Topic()))
	}
}

// The following methods were removed as they are no longer needed in mqtt-sensor-exporter:
// - subscribeToZigbee2MQTT (TopicPrefix removed, use Topics field instead)
// - subscribeToTasmotaFallback (TopicPrefix removed, use Topics field instead)
// - PublishCommand (command publishing moved to mqtt-device-manager)
// - PublishTasmotaCommand (command publishing moved to mqtt-device-manager)
// - TriggerDeviceDiscovery (device discovery moved to mqtt-device-manager)
