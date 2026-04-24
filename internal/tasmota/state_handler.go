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
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	iotv1alpha1 "github.com/hauke-cloud/kubernetes-iot-api/api/v1alpha1"
)

// StateHandler processes Tasmota state messages
type StateHandler struct {
	client client.Client
	log    *zap.Logger
}

// NewStateHandler creates a new state handler
func NewStateHandler(c client.Client, log *zap.Logger) *StateHandler {
	return &StateHandler{
		client: c,
		log:    log,
	}
}

// HandleMessage processes a state message
func (h *StateHandler) HandleMessage(ctx context.Context, msgCtx *MessageContext, payload []byte) error {
	var msg StateMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		h.log.Error("Failed to parse state message",
			zap.String("topic", msgCtx.Topic),
			zap.Error(err))
		return err
	}

	h.log.Debug("Processing state message",
		zap.String("bridge", msgCtx.BridgeName),
		zap.String("uptime", msg.Uptime))

	// Update MQTTBridge status
	return h.updateBridgeStatus(ctx, msgCtx, &msg)
}

// updateBridgeStatus updates the MQTTBridge CR status
func (h *StateHandler) updateBridgeStatus(ctx context.Context, msgCtx *MessageContext, state *StateMessage) error {
	// Fetch the MQTTBridge CR
	bridge := &iotv1alpha1.MQTTBridge{}
	bridgeKey := types.NamespacedName{
		Name:      msgCtx.BridgeName,
		Namespace: msgCtx.BridgeNamespace,
	}

	if err := h.client.Get(ctx, bridgeKey, bridge); err != nil {
		return err
	}

	// Update status
	now := metav1.Now()
	bridge.Status.ConnectionState = "Connected"
	bridge.Status.LastConnectedTime = &now

	// Add informational message
	if state.Wifi != nil {
		bridge.Status.Message = "Connected (WiFi RSSI: " + string(rune(state.Wifi.RSSI)) + ")"
	} else {
		bridge.Status.Message = "Connected (Uptime: " + state.Uptime + ")"
	}

	// Update the status
	if err := h.client.Status().Update(ctx, bridge); err != nil {
		h.log.Error("Failed to update bridge status",
			zap.String("bridge", msgCtx.BridgeName),
			zap.Error(err))
		return err
	}

	h.log.Debug("Updated bridge status",
		zap.String("bridge", msgCtx.BridgeName),
		zap.String("uptime", state.Uptime))

	return nil
}
