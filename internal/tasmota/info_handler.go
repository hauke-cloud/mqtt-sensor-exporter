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

// InfoHandler processes Tasmota info messages
type InfoHandler struct {
	client client.Client
	log    *zap.Logger
}

// NewInfoHandler creates a new info handler
func NewInfoHandler(c client.Client, log *zap.Logger) *InfoHandler {
	return &InfoHandler{
		client: c,
		log:    log,
	}
}

// HandleMessage processes an info message
func (h *InfoHandler) HandleMessage(ctx context.Context, msgCtx *MessageContext, payload []byte) error {
	var msg InfoMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		h.log.Error("Failed to parse info message",
			zap.String("topic", msgCtx.Topic),
			zap.Error(err))
		return err
	}

	h.log.Debug("Processing info message",
		zap.String("bridge", msgCtx.BridgeName),
		zap.String("version", msg.Version),
		zap.String("module", msg.Module))

	// TODO: Store Tasmota version and module info in MQTTBridge status or annotations

	return nil
}
