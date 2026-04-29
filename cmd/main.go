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

package main

import (
	"context"
	"crypto/tls"
	"flag"
	"os"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	uberzap "go.uber.org/zap"
	"gorm.io/gorm"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	iotv1alpha1 "github.com/hauke-cloud/kubernetes-iot-api/api/v1alpha1"
	"github.com/hauke-cloud/mqtt-sensor-exporter/internal/api"
	"github.com/hauke-cloud/mqtt-sensor-exporter/internal/database"
	"github.com/hauke-cloud/mqtt-sensor-exporter/internal/mqtt"
	"github.com/hauke-cloud/mqtt-sensor-exporter/internal/watcher"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(iotv1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

// nolint:gocyclo
func main() {
	var metricsAddr string
	var metricsCertPath, metricsCertName, metricsCertKey string
	var webhookCertPath, webhookCertName, webhookCertKey string
	var apiBindAddress string
	var enableLeaderElection bool
	var probeAddr string
	var secureMetrics bool
	var enableHTTP2 bool
	var tlsOpts []func(*tls.Config)
	flag.StringVar(&metricsAddr, "metrics-bind-address", "0", "The address the metrics endpoint binds to. "+
		"Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable the metrics service.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", true,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	flag.StringVar(&webhookCertPath, "webhook-cert-path", "", "The directory that contains the webhook certificate.")
	flag.StringVar(&webhookCertName, "webhook-cert-name", "tls.crt", "The name of the webhook certificate file.")
	flag.StringVar(&webhookCertKey, "webhook-cert-key", "tls.key", "The name of the webhook key file.")
	flag.StringVar(&metricsCertPath, "metrics-cert-path", "",
		"The directory that contains the metrics server certificate.")
	flag.StringVar(&metricsCertName, "metrics-cert-name", "tls.crt", "The name of the metrics server certificate file.")
	flag.StringVar(&metricsCertKey, "metrics-cert-key", "tls.key", "The name of the metrics server key file.")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	// API server flags
	flag.StringVar(&apiBindAddress, "api-bind-address", ":8111", "The address the REST API server binds to (HTTP).")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("Disabling HTTP/2")
		c.NextProtos = []string{"http/1.1"}
	}

	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	// Initial webhook TLS options
	webhookTLSOpts := tlsOpts
	webhookServerOptions := webhook.Options{
		TLSOpts: webhookTLSOpts,
	}

	if len(webhookCertPath) > 0 {
		setupLog.Info("Initializing webhook certificate watcher using provided certificates",
			"webhook-cert-path", webhookCertPath, "webhook-cert-name", webhookCertName, "webhook-cert-key", webhookCertKey)

		webhookServerOptions.CertDir = webhookCertPath
		webhookServerOptions.CertName = webhookCertName
		webhookServerOptions.KeyName = webhookCertKey
	}

	webhookServer := webhook.NewServer(webhookServerOptions)

	// Metrics endpoint is enabled in 'config/default/kustomization.yaml'. The Metrics options configure the server.
	// More info:
	// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.23.3/pkg/metrics/server
	// - https://book.kubebuilder.io/reference/metrics.html
	metricsServerOptions := metricsserver.Options{
		BindAddress:   metricsAddr,
		SecureServing: secureMetrics,
		TLSOpts:       tlsOpts,
	}

	if secureMetrics {
		// FilterProvider is used to protect the metrics endpoint with authn/authz.
		// These configurations ensure that only authorized users and service accounts
		// can access the metrics endpoint. The RBAC are configured in 'config/rbac/kustomization.yaml'. More info:
		// https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.23.3/pkg/metrics/filters#WithAuthenticationAndAuthorization
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	// If the certificate is not specified, controller-runtime will automatically
	// generate self-signed certificates for the metrics server. While convenient for development and testing,
	// this setup is not recommended for production.
	//
	// TODO(user): If you enable certManager, uncomment the following lines:
	// - [METRICS-WITH-CERTS] at config/default/kustomization.yaml to generate and use certificates
	// managed by cert-manager for the metrics server.
	// - [PROMETHEUS-WITH-CERTS] at config/prometheus/kustomization.yaml for TLS certification.
	if len(metricsCertPath) > 0 {
		setupLog.Info("Initializing metrics certificate watcher using provided certificates",
			"metrics-cert-path", metricsCertPath, "metrics-cert-name", metricsCertName, "metrics-cert-key", metricsCertKey)

		metricsServerOptions.CertDir = metricsCertPath
		metricsServerOptions.CertName = metricsCertName
		metricsServerOptions.KeyName = metricsCertKey
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsServerOptions,
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "5cf71cc2.hauke.cloud",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "Failed to start manager")
		os.Exit(1)
	}

	// Create MQTT BridgeManager
	zapLog, err := uberzap.NewDevelopment()
	if err != nil {
		setupLog.Error(err, "Failed to create zap logger")
		os.Exit(1)
	}
	defer func() {
		_ = zapLog.Sync()
	}()

	// Create Database Manager
	dbManager := database.NewManager(mgr.GetClient(), zapLog.With(uberzap.String("component", "database")))
	setupLog.Info("Created Database Manager")

	// Create MQTT BridgeManager with database manager
	mqttManager := mqtt.NewBridgeManager(mgr.GetClient(), zapLog, dbManager)
	setupLog.Info("Created MQTT BridgeManager")

	// Get current namespace
	namespace := os.Getenv("POD_NAMESPACE")
	if namespace == "" {
		namespace = "default"
	}

	// Set up Database watcher
	dbWatcher := watcher.NewDatabaseWatcher(
		mgr.GetClient(),
		zapLog.With(uberzap.String("component", "database-watcher")),
		namespace,
		func(ctx context.Context, db *iotv1alpha1.Database) error {
			return dbManager.Connect(ctx, db)
		},
	)
	if err := ctrl.NewControllerManagedBy(mgr).
		For(&iotv1alpha1.Database{}).
		WithEventFilter(dbWatcher.GetPredicate()).
		Complete(dbWatcher); err != nil {
		setupLog.Error(err, "Failed to create Database watcher")
		os.Exit(1)
	}
	setupLog.Info("Set up Database watcher")

	// Set up MQTTBridge watcher
	bridgeWatcher := watcher.NewMQTTBridgeWatcher(
		mgr.GetClient(),
		zapLog.With(uberzap.String("component", "mqttbridge-watcher")),
		namespace,
		func(ctx context.Context, bridge *iotv1alpha1.MQTTBridge) error {
			return mqttManager.Connect(ctx, bridge)
		},
	)
	if err := ctrl.NewControllerManagedBy(mgr).
		For(&iotv1alpha1.MQTTBridge{}).
		WithEventFilter(bridgeWatcher.GetPredicate()).
		Complete(bridgeWatcher); err != nil {
		setupLog.Error(err, "Failed to create MQTTBridge watcher")
		os.Exit(1)
	}
	setupLog.Info("Set up MQTTBridge watcher")

	// Set up signal handler once for the entire application
	ctx := ctrl.SetupSignalHandler()

	// Set up REST API server if configured
	var apiServer *api.Server
	if apiBindAddress != "" && apiBindAddress != "0" {
		// Get database connection from manager for API service
		// We'll need to wait for at least one database connection
		setupLog.Info("Configuring REST API server",
			"address", apiBindAddress)

		// Create a function to get DB connection for a specific sensor type
		getDB := func(sensorType string) *gorm.DB {
			// Get the database connection for this sensor type from the manager
			return dbManager.GetDBForSensorType(sensorType)
		}

		// Create API components
		// Pass a function that can dynamically retrieve the database connection based on sensor type
		alertsService := api.NewAlertsService(mgr.GetClient(), getDB, zapLog.With(uberzap.String("component", "api-alerts")))
		apiHandler := api.NewHandler(alertsService, zapLog.With(uberzap.String("component", "api-handler")))

		apiConfig := api.ServerConfig{
			Address: apiBindAddress,
		}

		apiServer = api.NewServer(apiConfig, apiHandler, zapLog.With(uberzap.String("component", "api-server")))

		// Start API server in background
		go func() {
			// Wait a bit for database to be ready
			time.Sleep(5 * time.Second)

			setupLog.Info("Starting REST API server")
			if err := apiServer.Start(ctx); err != nil {
				setupLog.Error(err, "API server stopped with error")
			}
		}()

		setupLog.Info("REST API server will start after database connection is established")
	}

	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "Failed to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "Failed to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("Starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "Failed to run manager")
		os.Exit(1)
	}
}
