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
	"encoding/json"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mqttv1alpha1 "github.com/hauke-cloud/mqtt-sensor-exporter/api/v1alpha1"
	"github.com/hauke-cloud/mqtt-sensor-exporter/internal/database"
)

// TelemetryHandler processes Tasmota telemetry messages
type TelemetryHandler struct {
	client    client.Client
	log       *zap.Logger
	dbManager *database.Manager
}

// NewTelemetryHandler creates a new telemetry handler
func NewTelemetryHandler(c client.Client, log *zap.Logger, dbManager *database.Manager) *TelemetryHandler {
	return &TelemetryHandler{
		client:    c,
		log:       log,
		dbManager: dbManager,
	}
}

// HandleMessage processes a telemetry message
func (h *TelemetryHandler) HandleMessage(ctx context.Context, msgCtx *MessageContext, payload []byte) error {
	var msg TelemetryMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		h.log.Error("Failed to parse telemetry message",
			zap.String("topic", msgCtx.Topic),
			zap.Error(err))
		return err
	}

	h.log.Debug("Processing telemetry message",
		zap.String("bridge", msgCtx.BridgeName),
		zap.Int("devices", len(msg.ZbReceived)))

	// Process each Zigbee device in the ZbReceived payload
	// The key in ZbReceived is the short address (e.g., "0x4F2E")
	for shortAddr, device := range msg.ZbReceived {
		if err := h.processZigbeeDevice(ctx, msgCtx, shortAddr, &device); err != nil {
			h.log.Error("Failed to process Zigbee device",
				zap.String("short_addr", shortAddr),
				zap.Error(err))
			// Continue processing other devices
			continue
		}
	}

	return nil
}

// processZigbeeDevice updates an existing Device CR for a Zigbee device
// Devices should only be created by discovery messages, not telemetry
func (h *TelemetryHandler) processZigbeeDevice(ctx context.Context, msgCtx *MessageContext, shortAddr string, device *ZigbeeDevice) error {
	// Find device by short address in status
	// List all devices and find the one with matching shortAddr
	deviceList := &mqttv1alpha1.DeviceList{}
	if err := h.client.List(ctx, deviceList, client.InNamespace(msgCtx.BridgeNamespace)); err != nil {
		return err
	}

	var existingDevice *mqttv1alpha1.Device
	for i := range deviceList.Items {
		if deviceList.Items[i].Status.ShortAddr == shortAddr {
			existingDevice = &deviceList.Items[i]
			break
		}
	}

	if existingDevice == nil {
		// Device not found - this is normal, it needs to be discovered first
		h.log.Debug("Device not found for telemetry update, skipping (device needs to be discovered first)",
			zap.String("shortAddr", shortAddr),
			zap.String("name", device.Name),
			zap.String("bridge", msgCtx.BridgeName))
		return nil
	}

	// Device exists, update status only
	return h.updateDevice(ctx, existingDevice, device)
}

// updateDevice updates an existing Device CR with telemetry data
func (h *TelemetryHandler) updateDevice(ctx context.Context, device *mqttv1alpha1.Device, zbDevice *ZigbeeDevice) error {
	// Update status
	h.updateDeviceStatus(device, zbDevice)

	if err := h.client.Status().Update(ctx, device); err != nil {
		return err
	}

	h.log.Debug("Updated Device CR",
		zap.String("device", device.Name),
		zap.String("ieee_addr", device.Spec.IEEEAddr))

	// Store measurement to database if device has a sensorType configured
	if device.Spec.SensorType != "" && h.dbManager != nil {
		if err := h.storeMeasurement(ctx, device, zbDevice); err != nil {
			h.log.Error("Failed to store measurement to database",
				zap.String("device", device.Name),
				zap.String("sensorType", device.Spec.SensorType),
				zap.Error(err))
			// Don't fail the update if database storage fails
		}
	}

	return nil
}

// storeMeasurement stores the measurement to the database with corrections applied
func (h *TelemetryHandler) storeMeasurement(ctx context.Context, device *mqttv1alpha1.Device, zbDevice *ZigbeeDevice) error {
	// Build payload from Zigbee device data
	payload := make(map[string]any)

	// Always include device identifiers
	if device.Status.ShortAddr != "" {
		payload["Device"] = device.Status.ShortAddr
	}
	if device.Spec.FriendlyName != "" {
		payload["Name"] = device.Spec.FriendlyName
	} else if zbDevice.Name != "" {
		payload["Name"] = zbDevice.Name
	}

	// Add measurements with corrections applied
	if zbDevice.Temperature != nil {
		correctedValue := applyCorrectionToFloat(*zbDevice.Temperature, "temperature", device)
		payload["Temperature"] = correctedValue
	}
	if zbDevice.Humidity != nil {
		correctedValue := applyCorrectionToFloat(*zbDevice.Humidity, "humidity", device)
		payload["Humidity"] = correctedValue
	}
	if zbDevice.Pressure != nil {
		correctedValue := applyCorrectionToFloat(*zbDevice.Pressure, "pressure", device)
		payload["Pressure"] = correctedValue
	}
	if zbDevice.Voltage != nil {
		correctedValue := applyCorrectionToFloat(*zbDevice.Voltage, "voltage", device)
		payload["Voltage"] = correctedValue
	}
	if zbDevice.BatteryPercentage != nil {
		// Battery percentage correction applied as int
		correctedValue := applyCorrectionToInt(*zbDevice.BatteryPercentage, "battery_percentage", device)
		payload["BatteryPercentage"] = float64(correctedValue)
	}
	if zbDevice.Power != nil {
		// Power correction applied as int
		correctedValue := applyCorrectionToInt(*zbDevice.Power, "power", device)
		payload["Power"] = float64(correctedValue)
	}
	if zbDevice.LinkQuality != nil {
		// Link quality correction applied as int
		correctedValue := applyCorrectionToInt(*zbDevice.LinkQuality, "link_quality", device)
		payload["LinkQuality"] = float64(correctedValue)
	}
	if zbDevice.Endpoint != nil {
		payload["Endpoint"] = float64(*zbDevice.Endpoint)
	}
	if zbDevice.Contact != nil {
		payload["Contact"] = *zbDevice.Contact
	}
	if zbDevice.Occupancy != nil {
		payload["Occupancy"] = *zbDevice.Occupancy
	}
	if zbDevice.WaterLeak != nil {
		payload["WaterLeak"] = *zbDevice.WaterLeak
	}

	// Store to database
	err := h.dbManager.StoreMeasurement(ctx, device.Name, device.Spec.SensorType, payload)
	if err != nil {
		return err
	}

	h.log.Debug("Stored measurement to database",
		zap.String("device", device.Name),
		zap.String("sensorType", device.Spec.SensorType),
		zap.Int("payloadSize", len(payload)))

	return nil
}

// updateDeviceStatus updates the device status from Zigbee device data
func (h *TelemetryHandler) updateDeviceStatus(device *mqttv1alpha1.Device, zbDevice *ZigbeeDevice) {
	now := metav1.Now()
	device.Status.LastSeen = &now
	device.Status.Available = true

	// Update link quality
	if zbDevice.LinkQuality != nil {
		lq := int32(*zbDevice.LinkQuality)
		device.Status.LinkQuality = &lq
	}

	// Update battery level
	if zbDevice.BatteryPercentage != nil {
		battery := int32(*zbDevice.BatteryPercentage)
		device.Status.BatteryLevel = &battery
	}

	// Update power state - store separately for stable monitoring
	if zbDevice.Power != nil {
		powerState := int32(*zbDevice.Power)
		device.Status.LastPowerState = &powerState
	}

	// Build measurement data with corrections applied
	measurements := make(map[string]any)

	if zbDevice.Temperature != nil {
		correctedValue := applyCorrectionToFloat(*zbDevice.Temperature, "temperature", device)
		measurements["temperature"] = correctedValue
	}
	if zbDevice.Humidity != nil {
		correctedValue := applyCorrectionToFloat(*zbDevice.Humidity, "humidity", device)
		measurements["humidity"] = correctedValue
	}
	if zbDevice.Pressure != nil {
		correctedValue := applyCorrectionToFloat(*zbDevice.Pressure, "pressure", device)
		measurements["pressure"] = correctedValue
	}
	if zbDevice.Voltage != nil {
		correctedValue := applyCorrectionToFloat(*zbDevice.Voltage, "voltage", device)
		measurements["voltage"] = correctedValue
	}
	if zbDevice.Power != nil {
		// Power correction applied as int
		correctedValue := applyCorrectionToInt(*zbDevice.Power, "power", device)
		measurements["power"] = correctedValue
	}
	if zbDevice.Contact != nil {
		measurements["contact"] = *zbDevice.Contact
	}
	if zbDevice.Occupancy != nil {
		measurements["occupancy"] = *zbDevice.Occupancy
	}
	if zbDevice.WaterLeak != nil {
		measurements["water_leak"] = *zbDevice.WaterLeak
	}
	if zbDevice.LinkQuality != nil {
		correctedValue := applyCorrectionToInt(*zbDevice.LinkQuality, "link_quality", device)
		measurements["link_quality"] = correctedValue
	}
	if zbDevice.Endpoint != nil {
		measurements["endpoint"] = *zbDevice.Endpoint
	}

	// Convert to JSON string
	if len(measurements) > 0 {
		if jsonData, err := json.Marshal(measurements); err == nil {
			device.Status.LastMeasurement = string(jsonData)
		}
	}

	// Extract capabilities from available measurements
	capabilities := []string{}
	if zbDevice.Temperature != nil {
		capabilities = append(capabilities, "temperature")
	}
	if zbDevice.Humidity != nil {
		capabilities = append(capabilities, "humidity")
	}
	if zbDevice.Pressure != nil {
		capabilities = append(capabilities, "pressure")
	}
	if zbDevice.Contact != nil {
		capabilities = append(capabilities, "contact")
	}
	if zbDevice.Occupancy != nil {
		capabilities = append(capabilities, "occupancy")
	}
	if zbDevice.WaterLeak != nil {
		capabilities = append(capabilities, "water_leak")
	}
	if zbDevice.Power != nil {
		capabilities = append(capabilities, "power")
	}

	device.Status.Capabilities = capabilities

	// Evaluate alert conditions on the corrected measurements
	device.Status.Alert = checkAlertConditions(measurements, device)
}
