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
	"sigs.k8s.io/controller-runtime/pkg/client"

	iotv1alpha1 "github.com/hauke-cloud/kubernetes-iot-api/api/v1alpha1"
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
	deviceList := &iotv1alpha1.DeviceList{}
	if err := h.client.List(ctx, deviceList, client.InNamespace(msgCtx.BridgeNamespace)); err != nil {
		return err
	}

	var existingDevice *iotv1alpha1.Device
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

// updateDevice stores sensor measurement to database without modifying Device CR
// mqtt-sensor-exporter is read-only and does not update device status
func (h *TelemetryHandler) updateDevice(ctx context.Context, device *iotv1alpha1.Device, zbDevice *ZigbeeDevice) error {
	h.log.Debug("Processing telemetry for device",
		zap.String("device", device.Name),
		zap.String("ieee_addr", device.Spec.IEEEAddr))

	// Store measurement to database if device has a sensorType configured
	if device.Spec.SensorType != "" && h.dbManager != nil {
		if err := h.storeMeasurement(ctx, device, zbDevice); err != nil {
			h.log.Error("Failed to store measurement to database",
				zap.String("device", device.Name),
				zap.String("sensorType", device.Spec.SensorType),
				zap.Error(err))
			return err
		}
	}

	return nil
}

// storeMeasurement stores the measurement to the database with corrections applied
func (h *TelemetryHandler) storeMeasurement(ctx context.Context, device *iotv1alpha1.Device, zbDevice *ZigbeeDevice) error {
	// Build payload from Zigbee device data
	payload := make(map[string]any)

	// Always include device identifiers - use zbDevice.Device directly as it always has the short address
	if zbDevice.Device != "" {
		payload["Device"] = zbDevice.Device
	}
	if device.Spec.FriendlyName != "" {
		payload["Name"] = device.Spec.FriendlyName
	} else if zbDevice.Name != "" {
		payload["Name"] = zbDevice.Name
	}

	h.log.Debug("Building database payload",
		zap.String("device", device.Name),
		zap.String("sensorType", device.Spec.SensorType),
		zap.String("shortAddr", zbDevice.Device))

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
	if zbDevice.WaterLevel != nil {
		// Water level correction applied as int
		correctedValue := applyCorrectionToInt(*zbDevice.WaterLevel, "water_level", device)
		payload["WaterLevel"] = float64(correctedValue)
	}
	if zbDevice.LastValveOpenDuration != nil {
		// Duration correction applied as int
		correctedValue := applyCorrectionToInt(*zbDevice.LastValveOpenDuration, "last_valve_open_duration", device)
		payload["LastValveOpenDuration"] = float64(correctedValue)
	}
	if zbDevice.IrrigationStartTime != nil {
		payload["IrrigationStartTime"] = float64(*zbDevice.IrrigationStartTime)
	}
	if zbDevice.IrrigationEndTime != nil {
		payload["IrrigationEndTime"] = float64(*zbDevice.IrrigationEndTime)
	}
	if zbDevice.DailyIrrigationVolume != nil {
		// Volume correction applied as int
		correctedValue := applyCorrectionToInt(*zbDevice.DailyIrrigationVolume, "daily_irrigation_volume", device)
		payload["DailyIrrigationVolume"] = float64(correctedValue)
	}

	// Store to database
	h.log.Debug("Attempting to store measurement to database",
		zap.String("device", device.Name),
		zap.String("sensorType", device.Spec.SensorType),
		zap.Int("payloadSize", len(payload)))

	err := h.dbManager.StoreMeasurement(ctx, device.Name, device.Spec.SensorType, payload)
	if err != nil {
		h.log.Error("StoreMeasurement returned error",
			zap.String("device", device.Name),
			zap.String("sensorType", device.Spec.SensorType),
			zap.Error(err))
		return err
	}

	h.log.Info("Successfully stored measurement to database",
		zap.String("device", device.Name),
		zap.String("sensorType", device.Spec.SensorType),
		zap.Int("payloadSize", len(payload)))

	return nil
}
