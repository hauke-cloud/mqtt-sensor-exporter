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

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	mqttv1alpha1 "github.com/hauke-cloud/mqtt-sensor-exporter/api/v1alpha1"
)

// DatabaseReconciler reconciles a Database object
type DatabaseReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=mqtt.hauke.cloud,resources=databases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mqtt.hauke.cloud,resources=databases/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=mqtt.hauke.cloud,resources=databases/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *DatabaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the Database instance
	database := &mqttv1alpha1.Database{}
	if err := r.Get(ctx, req.NamespacedName, database); err != nil {
		if errors.IsNotFound(err) {
			// Database was deleted
			log.Info("Database resource not found, ignoring")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get Database")
		return ctrl.Result{}, err
	}

	log.Info("Reconciling Database",
		"database", database.Name,
		"host", database.Spec.Host,
		"supportedSensorTypes", database.Spec.SupportedSensorTypes)

	// Update status to indicate we're initializing
	if database.Status.ConnectionState == "" {
		database.Status.ConnectionState = "Initializing"
		database.Status.Message = "Database controller is initializing connection"
		now := metav1.NewTime(time.Now())
		database.Status.LastConnectedTime = &now

		if err := r.Status().Update(ctx, database); err != nil {
			log.Error(err, "Failed to update Database status")
			return ctrl.Result{}, err
		}
		log.Info("Database status updated to Initializing", "database", database.Name)
	}

	// TODO: Implement actual database connection logic
	// For now, just mark as ready so users can see the resource is being managed
	if database.Status.ConnectionState == "Initializing" {
		database.Status.ConnectionState = "Disconnected"
		database.Status.Message = "Database manager not yet implemented - waiting for GORM integration"

		if err := r.Status().Update(ctx, database); err != nil {
			log.Error(err, "Failed to update Database status")
			return ctrl.Result{}, err
		}
		log.Info("Database status updated", "database", database.Name, "state", database.Status.ConnectionState)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DatabaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mqttv1alpha1.Database{}).
		Complete(r)
}
