# REST API Documentation

This document describes the REST API provided by the mqtt-sensor-exporter.

## Security

The API uses **mutual TLS (mTLS)** authentication, also known as client certificate authentication. This means:

1. **Server Certificate**: The server presents its TLS certificate to clients
2. **Client Certificate**: Clients must present a valid certificate signed by the configured CA
3. **Mutual Authentication**: Both parties verify each other's identity

### Configuration

The API server is configured via command-line flags:

```bash
--api-bind-address=":8443"              # Address to bind the API server
--api-tls-cert-path="/path/to/cert.pem" # Server TLS certificate
--api-tls-key-path="/path/to/key.pem"   # Server TLS private key
--api-client-ca-path="/path/to/ca.pem"  # CA for validating client certificates
--api-require-client-cert=true          # Require client certificates (default: true)
```

### Generating Certificates

For testing, you can generate self-signed certificates:

```bash
# Generate CA
openssl genrsa -out ca.key 4096
openssl req -new -x509 -days 365 -key ca.key -out ca.crt \
  -subj "/CN=Test CA"

# Generate server certificate
openssl genrsa -out server.key 4096
openssl req -new -key server.key -out server.csr \
  -subj "/CN=localhost"
openssl x509 -req -days 365 -in server.csr -CA ca.crt -CAkey ca.key \
  -CAcreateserial -out server.crt

# Generate client certificate
openssl genrsa -out client.key 4096
openssl req -new -key client.key -out client.csr \
  -subj "/CN=api-client"
openssl x509 -req -days 365 -in client.csr -CA ca.crt -CAkey ca.key \
  -CAcreateserial -out client.crt
```

### Making Requests

With curl:

```bash
curl --cert client.crt --key client.key --cacert ca.crt \
  https://localhost:8443/v2/api/alerts
```

With Go:

```go
cert, _ := tls.LoadX509KeyPair("client.crt", "client.key")
caCert, _ := os.ReadFile("ca.crt")
caCertPool := x509.NewCertPool()
caCertPool.AppendCertsFromPEM(caCert)

client := &http.Client{
    Transport: &http.Transport{
        TLSClientConfig: &tls.Config{
            Certificates: []tls.Certificate{cert},
            RootCAs:      caCertPool,
        },
    },
}

resp, err := client.Get("https://localhost:8443/v2/api/alerts")
```

## Endpoints

### GET /v2/api/alerts

Returns all devices that have triggered their configured alert thresholds.

**Authentication**: Client certificate required

**Response**: `200 OK`

```json
{
  "devices": [
    {
      "deviceId": "device-00158d000123abcd",
      "deviceName": "Garden Moisture Sensor",
      "sensorType": "moisture",
      "location": "Garden",
      "room": "Outdoor",
      "ieeeAddr": "0x00158d000123abcd",
      "shortAddr": "0xBF16",
      "alertCondition": {
        "measurement": "humidity",
        "operator": "below",
        "threshold": "20"
      },
      "currentValue": 15.5,
      "lastMeasurement": "2026-04-27T23:45:00Z",
      "triggeredAt": "2026-04-27T22:30:15Z"
    }
  ],
  "count": 1,
  "timestamp": "2026-04-27T23:50:00Z"
}
```

**Fields**:

- `deviceId`: Kubernetes resource name of the device
- `deviceName`: Friendly name of the device (optional)
- `sensorType`: Type of sensor (moisture, valve, water_level, room)
- `location`: Physical location of the device (optional)
- `room`: Room where the device is located (optional)
- `ieeeAddr`: IEEE address from Kubernetes Device resource
- `shortAddr`: Zigbee short address from database (optional)
- `alertCondition`: The configured alert condition
  - `measurement`: Name of the measured value (e.g., "temperature", "humidity")
  - `operator`: Comparison operator ("above", "below", "is")
  - `threshold`: Threshold value as string
- `currentValue`: Latest measured value that triggered the alert (optional)
- `lastMeasurement`: Timestamp of the last measurement (optional)
- `triggeredAt`: When the alert was first triggered (optional)

**Error Response**: `401 Unauthorized`

```json
{
  "error": "Unauthorized",
  "code": 401,
  "timestamp": "2026-04-27T23:50:00Z"
}
```

**Error Response**: `500 Internal Server Error`

```json
{
  "error": "Failed to retrieve alerts",
  "code": 500,
  "timestamp": "2026-04-27T23:50:00Z"
}
```

### GET /health

Health check endpoint (does not require client certificate for simple monitoring).

**Authentication**: None required

**Response**: `200 OK`

```json
{
  "status": "healthy",
  "time": "2026-04-27T23:50:00Z"
}
```

## Alert Conditions

Devices can be configured with alert conditions in their Kubernetes Device resource:

```yaml
apiVersion: iot.hauke.cloud/v1alpha1
kind: Device
metadata:
  name: garden-moisture-sensor
spec:
  sensorType: moisture
  ieeeAddr: "0x00158d000123abcd"
  alertCondition:
    measurement: "humidity"    # Field name to monitor
    operator: "below"          # above, below, or is
    value: "20"               # Threshold value
  friendlyName: "Garden Moisture Sensor"
  location: "Garden"
  room: "Outdoor"
```

When the measured value meets the alert condition, the Device's `status.alert` field is set to `true`, and the device will appear in the `/v2/api/alerts` endpoint response.

## Integration Examples

### Prometheus AlertManager

You can use the API as a webhook target for AlertManager:

```yaml
receivers:
  - name: 'sensor-alerts'
    webhook_configs:
      - url: 'https://mqtt-sensor-exporter:8443/v2/api/alerts'
        tls_config:
          cert_file: /certs/client.crt
          key_file: /certs/client.key
          ca_file: /certs/ca.crt
```

### Monitoring Script

Simple bash script to check for alerts:

```bash
#!/bin/bash

ALERTS=$(curl -s --cert client.crt --key client.key --cacert ca.crt \
  https://localhost:8443/v2/api/alerts)

COUNT=$(echo "$ALERTS" | jq '.count')

if [ "$COUNT" -gt 0 ]; then
  echo "WARNING: $COUNT device(s) have triggered alerts"
  echo "$ALERTS" | jq '.devices[] | "\(.deviceName): \(.alertCondition.measurement) \(.alertCondition.operator) \(.alertCondition.threshold) (current: \(.currentValue))"'
  exit 1
fi

echo "OK: No alerts triggered"
exit 0
```

## Rate Limiting

Currently, no rate limiting is implemented. In production environments, consider:

1. Using an API gateway with rate limiting
2. Implementing request throttling at the application level
3. Monitoring API usage patterns

## Future Endpoints

Planned endpoints for future releases:

- `GET /v2/api/devices` - List all devices
- `GET /v2/api/devices/{id}` - Get specific device details
- `GET /v2/api/measurements/{id}` - Get measurements for a device
- `GET /v2/api/metrics` - Prometheus-compatible metrics
