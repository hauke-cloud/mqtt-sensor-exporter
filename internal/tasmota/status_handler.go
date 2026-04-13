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
)

// StatusHandler processes Tasmota status/result messages
type StatusHandler struct {
	client           client.Client
	log              *zap.Logger
	discoveryHandler *DiscoveryHandler
}

// NewStatusHandler creates a new status handler
func NewStatusHandler(c client.Client, log *zap.Logger, discoveryHandler *DiscoveryHandler) *StatusHandler {
	return &StatusHandler{
		client:           c,
		log:              log,
		discoveryHandler: discoveryHandler,
	}
}

// HandleMessage processes a status message
func (h *StatusHandler) HandleMessage(ctx context.Context, msgCtx *MessageContext, payload []byte) error {
	var msg StatusMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		h.log.Error("Failed to parse status message",
			zap.String("topic", msgCtx.Topic),
			zap.Error(err))
		return err
	}

	h.log.Debug("Processing status message",
		zap.String("bridge", msgCtx.BridgeName))

	// Check for discovery messages (ZbStatus1 or ZbStatus3) and forward to discovery handler
	if (msg.ZbStatus1 != nil && len(msg.ZbStatus1) > 0) || (msg.ZbStatus3 != nil && len(msg.ZbStatus3) > 0) {
		if h.discoveryHandler != nil {
			h.log.Debug("Forwarding discovery message to discovery handler",
				zap.Bool("hasZbStatus1", msg.ZbStatus1 != nil),
				zap.Bool("hasZbStatus3", msg.ZbStatus3 != nil))
			return h.discoveryHandler.HandleMessage(ctx, msgCtx, payload)
		}
	}

	// Handle ZbSend results
	if msg.ZbSend != nil {
		h.handleZbSendResult(ctx, msgCtx, msg.ZbSend)
	}

	// Handle ZbName results
	if msg.ZbName != nil {
		h.handleZbNameResult(ctx, msgCtx, msg.ZbName)
	}

	return nil
}

// handleZbSendResult processes ZbSend command results
func (h *StatusHandler) handleZbSendResult(ctx context.Context, msgCtx *MessageContext, result *ZbSendResult) {
	h.log.Info("Received ZbSend result",
		zap.String("device", result.Device),
		zap.String("status", result.Status),
		zap.String("bridge", msgCtx.BridgeName))

	// TODO: Update Device CR or trigger callbacks based on command result
	// For now, just log the result
}

// handleZbNameResult processes ZbName command results
func (h *StatusHandler) handleZbNameResult(ctx context.Context, msgCtx *MessageContext, result *ZbNameResult) {
	h.log.Info("Received ZbName result",
		zap.String("device", result.Device),
		zap.String("name", result.Name),
		zap.String("bridge", msgCtx.BridgeName))

	// TODO: Update Device CR friendly name if needed
}
