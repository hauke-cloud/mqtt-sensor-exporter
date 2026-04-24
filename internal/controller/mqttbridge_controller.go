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

package controller

import (
	"context"
	"time"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	iotv1alpha1 "github.com/hauke-cloud/kubernetes-iot-api/api/v1alpha1"
	"github.com/hauke-cloud/mqtt-sensor-exporter/internal/mqtt"
)

const (
	mqttBridgeFinalizer    = "mqtt.hauke.cloud/finalizer"
	connectionRetryTimeout = 30 * time.Second
	discoveryInterval      = 30 * time.Second
)

// MQTTBridgeReconciler reconciles a MQTTBridge object
type MQTTBridgeReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	Log         *zap.Logger
	MQTTManager *mqtt.BridgeManager
}

// +kubebuilder:rbac:groups=mqtt.hauke.cloud,resources=mqttbridges,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mqtt.hauke.cloud,resources=mqttbridges/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=mqtt.hauke.cloud,resources=mqttbridges/finalizers,verbs=update
// +kubebuilder:rbac:groups=mqtt.hauke.cloud,resources=devices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mqtt.hauke.cloud,resources=devices/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch

// Reconcile handles MQTTBridge CR changes
func (r *MQTTBridgeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.With(
		zap.String("namespace", req.Namespace),
		zap.String("name", req.Name))

	log.Info("Reconciling MQTTBridge")

	// Fetch the MQTTBridge instance
	bridge := &iotv1alpha1.MQTTBridge{}
	err := r.Get(ctx, req.NamespacedName, bridge)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, could have been deleted
			log.Info("MQTTBridge not found, cleaning up MQTT connection")
			r.MQTTManager.Disconnect(req.Namespace, req.Name)
			return ctrl.Result{}, nil
		}
		// Error reading the object
		log.Error("Failed to get MQTTBridge", zap.Error(err))
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !bridge.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, bridge, log)
	}

	// Add finalizer if it doesn't exist
	if !controllerutil.ContainsFinalizer(bridge, mqttBridgeFinalizer) {
		controllerutil.AddFinalizer(bridge, mqttBridgeFinalizer)
		if err := r.Update(ctx, bridge); err != nil {
			log.Error("Failed to add finalizer", zap.Error(err))
			return ctrl.Result{}, err
		}
		log.Info("Added finalizer to MQTTBridge")
	}

	// Connect to MQTT broker
	if err := r.MQTTManager.Connect(ctx, bridge); err != nil {
		log.Error("Failed to connect to MQTT broker", zap.Error(err))
		// Update status
		bridge.Status.ConnectionState = "Error"
		bridge.Status.Message = err.Error()
		if statusErr := r.Status().Update(ctx, bridge); statusErr != nil {
			log.Error("Failed to update status", zap.Error(statusErr))
		}
		// Retry after configured timeout
		return ctrl.Result{RequeueAfter: connectionRetryTimeout}, err
	}

	// Update status to connected
	if r.MQTTManager.IsConnected(bridge.Namespace, bridge.Name) {
		bridge.Status.ConnectionState = "Connected"
		bridge.Status.Message = "Successfully connected to MQTT broker"
		if err := r.Status().Update(ctx, bridge); err != nil {
			log.Error("Failed to update status", zap.Error(err))
			return ctrl.Result{}, err
		}
		log.Info("MQTTBridge connected successfully")

		// Trigger device discovery if enabled
		if bridge.Spec.DiscoveryEnabled != nil && *bridge.Spec.DiscoveryEnabled {
			if err := r.MQTTManager.TriggerDeviceDiscovery(bridge.Namespace, bridge.Name); err != nil {
				log.Error("Failed to trigger device discovery", zap.Error(err))
			} else {
				log.Debug("Triggered device discovery")
			}
		}
	}

	// Requeue after discovery interval to trigger periodic discovery
	requeueAfter := discoveryInterval
	if bridge.Spec.DiscoveryEnabled != nil && !*bridge.Spec.DiscoveryEnabled {
		// If discovery is disabled, don't need frequent requeues
		requeueAfter = 5 * time.Minute
	}

	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

// handleDeletion handles the deletion of MQTTBridge
func (r *MQTTBridgeReconciler) handleDeletion(ctx context.Context, bridge *iotv1alpha1.MQTTBridge, log *zap.Logger) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(bridge, mqttBridgeFinalizer) {
		log.Info("Disconnecting MQTT bridge")

		// Disconnect from MQTT broker
		r.MQTTManager.Disconnect(bridge.Namespace, bridge.Name)

		// Remove finalizer
		controllerutil.RemoveFinalizer(bridge, mqttBridgeFinalizer)
		if err := r.Update(ctx, bridge); err != nil {
			log.Error("Failed to remove finalizer", zap.Error(err))
			return ctrl.Result{}, err
		}
		log.Info("Removed finalizer and disconnected MQTT bridge")
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MQTTBridgeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&iotv1alpha1.MQTTBridge{}).
		Named("mqttbridge").
		Complete(r)
}
