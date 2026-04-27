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

const (
	serviceName = "mqtt-sensor-exporter"
)

// DatabaseWatcher watches Database resources and registers this service
type DatabaseWatcher struct {
	client    client.Client
	log       *zap.Logger
	namespace string
	onConnect func(ctx context.Context, db *iotv1alpha1.Database) error
}

// NewDatabaseWatcher creates a new Database watcher
func NewDatabaseWatcher(c client.Client, log *zap.Logger, namespace string, onConnect func(ctx context.Context, db *iotv1alpha1.Database) error) *DatabaseWatcher {
	return &DatabaseWatcher{
		client:    c,
		log:       log,
		namespace: namespace,
		onConnect: onConnect,
	}
}

// Reconcile handles Database resource changes
func (w *DatabaseWatcher) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := w.log.With(zap.String("database", req.String()))

	// Fetch the Database
	database := &iotv1alpha1.Database{}
	if err := w.client.Get(ctx, req.NamespacedName, database); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Call onConnect callback
	if w.onConnect != nil {
		if err := w.onConnect(ctx, database); err != nil {
			log.Error("Failed to connect to database", zap.Error(err))
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// GetPredicate returns a predicate that only triggers on generation changes
func (w *DatabaseWatcher) GetPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return true // Always handle new databases
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldDB, oldOk := e.ObjectOld.(*iotv1alpha1.Database)
			newDB, newOk := e.ObjectNew.(*iotv1alpha1.Database)
			if !oldOk || !newOk {
				return false
			}
			// Only trigger on generation changes (spec updates)
			return oldDB.Generation != newDB.Generation
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false // We don't need to handle deletions
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}
