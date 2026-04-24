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

package mqtt

// Example usage:
//
// In your controller setup (internal/controller/mqttbridge_controller.go):
//
// import (
//     "go.uber.org/zap"
//     "github.com/hauke-cloud/mqtt-sensor-exporter/internal/mqtt"
// )
//
// func (r *MQTTBridgeReconciler) SetupWithManager(mgr ctrl.Manager) error {
//     // Get zap logger from controller-runtime
//     zapLog, err := zap.NewProduction()
//     if err != nil {
//         return err
//     }
//     defer zapLog.Sync()
//
//     // Or for development:
//     // zapLog, err := zap.NewDevelopment()
//
//     // Create MQTT manager with zap logger
//     r.mqttManager = mqtt.NewBridgeManager(mgr.GetClient(), zapLog)
//
//     // Set callbacks
//     r.mqttManager.SetCallbacks(
//         r.handleDeviceDiscovery,
//         r.handleMeasurement,
//     )
//
//     return ctrl.NewControllerManagedBy(mgr).
//         For(&iotv1alpha1.MQTTBridge{}).
//         Complete(r)
// }
//
// In your main.go, you can get the zap logger from the controller-runtime logger:
//
// import (
//     "go.uber.org/zap/zapcore"
//     "sigs.k8s.io/controller-runtime/pkg/log/zap"
// )
//
// func main() {
//     opts := zap.Options{
//         Development: true,
//         // Configure zap encoder for structured logging
//         EncoderConfigOptions: []zap.EncoderConfigOption{
//             func(ec *zapcore.EncoderConfig) {
//                 ec.EncodeTime = zapcore.ISO8601TimeEncoder
//                 ec.EncodeDuration = zapcore.StringDurationEncoder
//             },
//         },
//     }
//     opts.BindFlags(flag.CommandLine)
//     flag.Parse()
//
//     ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
//
//     // Get the raw zap logger for MQTT manager
//     zapLog := zap.NewRaw(zap.UseFlagOptions(&opts))
//     defer zapLog.Sync()
//
//     // Pass to controllers...
// }
