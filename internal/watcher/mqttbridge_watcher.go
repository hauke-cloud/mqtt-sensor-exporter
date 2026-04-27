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

package watcher

import (
	"context"

	"go.uber.org/zap"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	iotv1alpha1 "github.com/hauke-cloud/kubernetes-iot-api/api/v1alpha1"
)

// MQTTBridgeWatcher watches MQTTBridge resources and registers this service
type MQTTBridgeWatcher struct {
	client    client.Client
	log       *zap.Logger
	namespace string
	onConnect func(ctx context.Context, bridge *iotv1alpha1.MQTTBridge) error
}

// NewMQTTBridgeWatcher creates a new MQTTBridge watcher
func NewMQTTBridgeWatcher(c client.Client, log *zap.Logger, namespace string, onConnect func(ctx context.Context, bridge *iotv1alpha1.MQTTBridge) error) *MQTTBridgeWatcher {
	return &MQTTBridgeWatcher{
		client:    c,
		log:       log,
		namespace: namespace,
		onConnect: onConnect,
	}
}

// Reconcile handles MQTTBridge resource changes
func (w *MQTTBridgeWatcher) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := w.log.With(zap.String("mqttbridge", req.String()))

	// Fetch the MQTTBridge
	bridge := &iotv1alpha1.MQTTBridge{}
	if err := w.client.Get(ctx, req.NamespacedName, bridge); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Call onConnect callback
	if w.onConnect != nil {
		if err := w.onConnect(ctx, bridge); err != nil {
			log.Error("Failed to connect to MQTT bridge", zap.Error(err))
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// GetPredicate returns a predicate that triggers on new resources and generation changes
func (w *MQTTBridgeWatcher) GetPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return true // Always handle new bridges
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			// Trigger on any update to check if we need to register
			return true
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false // We don't need to handle deletions
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}
