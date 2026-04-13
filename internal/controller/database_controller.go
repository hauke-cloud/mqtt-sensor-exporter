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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mqttv1alpha1 "github.com/hauke-cloud/mqtt-sensor-exporter/api/v1alpha1"
	"github.com/hauke-cloud/mqtt-sensor-exporter/internal/database"
)

// DatabaseReconciler reconciles a Database object
type DatabaseReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	Log       *zap.Logger
	DBManager *database.Manager
}

// +kubebuilder:rbac:groups=mqtt.hauke.cloud,resources=databases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mqtt.hauke.cloud,resources=databases/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=mqtt.hauke.cloud,resources=databases/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *DatabaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.With(
		zap.String("namespace", req.Namespace),
		zap.String("name", req.Name))

	// Fetch the Database instance
	db := &mqttv1alpha1.Database{}
	if err := r.Get(ctx, req.NamespacedName, db); err != nil {
		if errors.IsNotFound(err) {
			// Database was deleted - disconnect
			log.Info("Database resource deleted, cleaning up connection")
			r.DBManager.Disconnect(req.Namespace, req.Name)
			return ctrl.Result{}, nil
		}
		log.Error("Failed to get Database", zap.Error(err))
		return ctrl.Result{}, err
	}

	log.Info("Reconciling Database",
		zap.String("database", db.Name),
		zap.String("host", db.Spec.Host),
		zap.Strings("supportedSensorTypes", db.Spec.SupportedSensorTypes))

	// Connect to database
	if err := r.DBManager.Connect(ctx, db); err != nil {
		log.Error("Failed to connect to database", zap.Error(err))

		// Update status to error
		db.Status.ConnectionState = "Error"
		db.Status.Message = err.Error()
		if statusErr := r.Status().Update(ctx, db); statusErr != nil {
			log.Error("Failed to update status", zap.Error(statusErr))
		}

		// Retry after 30 seconds
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	// Update status to connected
	db.Status.ConnectionState = "Connected"
	db.Status.Message = "Successfully connected to database"
	now := metav1.NewTime(time.Now())
	db.Status.LastConnectedTime = &now

	if err := r.Status().Update(ctx, db); err != nil {
		log.Error("Failed to update status", zap.Error(err))
		return ctrl.Result{}, err
	}

	log.Info("Database connected successfully",
		zap.String("database", db.Name),
		zap.String("host", db.Spec.Host),
		zap.Strings("supportedSensorTypes", db.Spec.SupportedSensorTypes))

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DatabaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mqttv1alpha1.Database{}).
		Complete(r)
}
