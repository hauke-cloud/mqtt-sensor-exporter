# REST API Documentation

## Overview

The mqtt-sensor-exporter provides a REST API for querying IoT sensor alerts. The API evaluates alert conditions **dynamically in real-time** by comparing device measurements against configured thresholds. 

**Key Features**:
- Real-time alert evaluation from database measurements
- No Device CRD status updates required
- Flexible time-based queries with `since` parameter
- Multiple filter options (device type, location, room)

The API runs as an HTTP server inside the Kubernetes cluster, with TLS termination and authentication handled by the ingress controller.

## Base URL

When deployed with ingress:
```
https://mqtt-api.example.com/api/v2
```

## Authentication

Authentication is handled by the ingress controller using client certificates (mTLS). Configure authentication via ingress annotations:

```yaml
nginx.ingress.kubernetes.io/auth-tls-verify-client: "on"
nginx.ingress.kubernetes.io/auth-tls-secret: "default/mqtt-api-client-ca"
```

## How Alert Evaluation Works

The API evaluates alert conditions **dynamically** by:

1. Fetching all devices with `alertCondition` configured
2. Querying the database for measurement values
3. Evaluating the condition against the measurement value
4. Returning devices where the condition is met

### Measurement Value Calculation

**Without `since` parameter** (default):
- Uses the **latest single measurement** value
- Example: Latest water_level = 27

**With `since` parameter**:
- Calculates the **average (AVG)** over the specified time window
- Uses SQL AVG to compute mean value
- Example: `since=5m` → Average water_level over last 5 minutes

```sql
-- Example query with since=5m
SELECT AVG(level) as avg_value, MAX(timestamp) as last_timestamp
FROM water_level_measurements
WHERE device_id = ? AND timestamp >= NOW() - INTERVAL '5 minutes'
```

### Supported Operators

- `above` - Measurement > Threshold
- `below` - Measurement < Threshold  
- `is` or `equals` - Measurement == Threshold

### Example Scenarios

**Scenario 1: Instant alert (no `since`)**
```yaml
# Device spec
alertCondition:
  measurement: water_level
  operator: below
  value: "30"

# Latest measurement: water_level = 27
# Evaluation: 27 < 30 → Alert triggered ✓
```

**Scenario 2: Average over time window**
```yaml
# Device spec
alertCondition:
  measurement: water_level
  operator: below
  value: "30"

# Request: GET /api/v2/alerts?since=5m
# Measurements in last 5 minutes: [25, 27, 26, 28, 24]
# Average: 26
# Evaluation: 26 < 30 → Alert triggered ✓
```

**Note**: The API does **not** rely on `device.Status.Alert`. Alert status is not persisted in the Device CRD.

## Endpoints

### GET /api/v2/alerts

Returns all devices that have triggered their configured alert thresholds.

**Query Parameters**:
- `device-type` (optional) - Filter by sensor type
  - Values: `valve`, `moisture`, `room`, `water_level`
- `location` (optional) - Filter by device location
- `room` (optional) - Filter by device room  
- `since` (optional) - Time window for alert evaluation
  - Examples: `1min`, `5m`, `1h`, `30s`, `1d`
  - **When provided**: Calculates **average** measurement over the time window
  - **Without `since`**: Uses the latest single measurement value

**Example Requests**:
```bash
# Get all alerts
curl https://mqtt-api.example.com/api/v2/alerts

# Get alerts for valve devices
curl https://mqtt-api.example.com/api/v2/alerts?device-type=valve

# Get alerts from the last 5 minutes
curl https://mqtt-api.example.com/api/v2/alerts?since=5m

# Combine multiple filters
curl https://mqtt-api.example.com/api/v2/alerts?device-type=valve&location=Garden&since=1h
```

**Response** (200 OK):
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
      "lastMeasurement": "2026-04-27T23:45:00Z"
    }
  ],
  "count": 1,
  "timestamp": "2026-04-27T23:50:00Z"
}
```

### GET /api/v2/alerts/{device-name}

Returns alert information for a specific device.

**Path Parameters**:
- `device-name` (required) - Kubernetes resource name of the device

**Example Request**:
```bash
curl https://mqtt-api.example.com/api/v2/alerts/device-00158d000123abcd
```

**Response** (200 OK):
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
      "lastMeasurement": "2026-04-27T23:45:00Z"
    }
  ],
  "count": 1,
  "timestamp": "2026-04-27T23:50:00Z"
}
```

**Error Response** (404 Not Found):
```json
{
  "error": "Device 'device-xyz' not found or has no active alerts",
  "code": 404,
  "timestamp": "2026-04-27T23:50:00Z"
}
```

### GET /health

Health check endpoint (no authentication required).

**Example Request**:
```bash
curl https://mqtt-api.example.com/health
```

**Response** (200 OK):
```json
{
  "status": "healthy",
  "time": "2026-04-27T23:50:00Z"
}
```

## Data Models

### AlertDevice

| Field | Type | Description |
|-------|------|-------------|
| deviceId | string | Kubernetes resource name |
| deviceName | string | Friendly name of the device |
| sensorType | string | Type of sensor (valve, moisture, etc.) |
| location | string | Physical location |
| room | string | Room name |
| ieeeAddr | string | IEEE address |
| shortAddr | string | Zigbee short address |
| alertCondition | object | Alert threshold configuration |
| currentValue | number | Current measurement value |
| lastMeasurement | timestamp | When last measured |

### AlertCondition

| Field | Type | Description |
|-------|------|-------------|
| measurement | string | Field being monitored |
| operator | string | Comparison operator (above, below, is) |
| threshold | string | Trigger value |

## Error Responses

All error responses follow this format:

```json
{
  "error": "Error message",
  "code": 400,
  "timestamp": "2026-04-27T23:50:00Z"
}
```

**HTTP Status Codes**:
- `200` - Success
- `400` - Bad Request (invalid parameters)
- `404` - Not Found (device not found or no alerts)
- `405` - Method Not Allowed
- `500` - Internal Server Error

## Usage Examples

### With curl and client certificate

```bash
curl --cert client.crt --key client.key \
  https://mqtt-api.example.com/api/v2/alerts
```

### With Python

```python
import requests

response = requests.get(
    'https://mqtt-api.example.com/api/v2/alerts',
    params={'device-type': 'valve', 'since': '5m'},
    cert=('client.crt', 'client.key'),
    verify='ca.crt'
)

data = response.json()
print(f"Found {data['count']} alerts")
for device in data['devices']:
    print(f"- {device['deviceName']}: {device['currentValue']}")
```

### With Go

```go
package main

import (
    "crypto/tls"
    "crypto/x509"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"
)

func main() {
    // Load client certificate
    cert, _ := tls.LoadX509KeyPair("client.crt", "client.key")
    
    // Load CA certificate
    caCert, _ := os.ReadFile("ca.crt")
    caCertPool := x509.NewCertPool()
    caCertPool.AppendCertsFromPEM(caCert)
    
    // Create HTTP client with TLS config
    client := &http.Client{
        Transport: &http.Transport{
            TLSClientConfig: &tls.Config{
                Certificates: []tls.Certificate{cert},
                RootCAs:      caCertPool,
            },
        },
    }
    
    // Make request
    resp, err := client.Get("https://mqtt-api.example.com/api/v2/alerts?device-type=valve")
    if err != nil {
        panic(err)
    }
    defer resp.Body.Close()
    
    // Parse response
    body, _ := io.ReadAll(resp.Body)
    var result map[string]interface{}
    json.Unmarshal(body, &result)
    
    fmt.Printf("Found %v alerts\n", result["count"])
}
```

## Filter Examples

### By Device Type

Get alerts for all valve devices:
```bash
GET /api/v2/alerts?device-type=valve
```

### By Location

Get alerts in the Garden:
```bash
GET /api/v2/alerts?location=Garden
```

### By Time Window

Get alerts from the last hour:
```bash
GET /api/v2/alerts?since=1h
```

Supported time units:
- `s` - seconds (e.g., `30s`)
- `m` - minutes (e.g., `5m`, `1min`)
- `h` - hours (e.g., `2h`, `1hour`)
- `d` - days (e.g., `1d`, `7d`)

### Combined Filters

Get valve alerts in the Garden from the last 30 minutes:
```bash
GET /api/v2/alerts?device-type=valve&location=Garden&since=30m
```

## Deployment

See the [Helm chart documentation](deployments/helm/mqtt-sensor-exporter/README.md) for deployment instructions with ingress and authentication configuration.

Example values for Helm:

```yaml
api:
  enabled: true
  ingress:
    enabled: true
    className: nginx
    annotations:
      cert-manager.io/cluster-issuer: "letsencrypt-prod"
      nginx.ingress.kubernetes.io/auth-tls-verify-client: "on"
      nginx.ingress.kubernetes.io/auth-tls-secret: "default/mqtt-api-client-ca"
    hosts:
      - host: mqtt-api.example.com
        paths:
          - path: /
            pathType: Prefix
    tls:
      - secretName: mqtt-api-tls
        hosts:
          - mqtt-api.example.com
```
