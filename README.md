<!-- llm-readme-management spec=1 commit=36210983f23b11e1a8a0ba69f7c29b3f1924b4e1 template=terraform model=qwen3.8-27b-q4 digest=f68682b55584 generated=2026-10-12T10:08:11Z -->
<a href="https://hauke.cloud" target="_blank"><img src="https://img.shields.io/badge/home-hauke.cloud-brightgreen" alt="hauke.cloud" style="display: block;" /></a>
<a href="https://github.com/hauke-cloud" target="_blank"><img src="https://img.shields.io/badge/github-hauke.cloud-blue" alt="hauke.cloud Github Organisation" style="display: block;" /></a>
<a href="https://github.com/hauke-cloud/llm-readme-management" target="_blank"><img src="https://img.shields.io/badge/template-terraform-orange" alt="Repository type - terraform" style="display: block;" /></a>


# Template Repository


<img src="https://raw.githubusercontent.com/hauke-cloud/.github/main/resources/img/organisation-logo-small.png" alt="hauke.cloud logo" width="109" height="123" align="right">


<llm header hint="Say whether this is a reusable module or a root module that owns real state.">

This is a Go Kubernetes operator that subscribes to sensor measurement topics on MQTT brokers (Tasmota Zigbee bridges) and stores the readings in PostgreSQL/TimescaleDB, exposing a small REST API for alert queries. It targets operators of a Kubernetes cluster running the hauke.cloud IoT stack, alongside the companion `mqtt-device-manager` project.

</llm>


## :book: Description

<llm description>

`mqtt-sensor-exporter` is a Kubernetes operator that ingests sensor measurements from MQTT brokers and persists them in PostgreSQL or TimescaleDB. It watches two custom resources in the `mqtt.hauke.cloud/v1alpha1` group: `MQTTBridge`, which describes a broker connection and the topics to subscribe to, and `Database`, which describes a target database and the sensor types it should store. For each bridge it opens a paho MQTT client, subscribes to `telemetry` and `sensor` topics, parses Tasmota `ZbReceived` payloads, and writes the resulting measurements through per-type GORM handlers.

A small HTTP API (default `:8111`) lets you query stored data and evaluate per-device alert conditions (`above`, `below`, `is`, with optional time-window averaging).

- Subscribes to Tasmota Zigbee-bridge topics; Zigbee2MQTT and generic MQTT are not yet implemented
- Stores measurements for four sensor types: `moisture`, `water_level`, `valve`, `room`
- Runs schema migrations for each supported sensor type when a `Database` CR is created
- Serves `GET /api/v2/alerts` (filterable by device type, location, room, time window) and `GET /api/v2/alerts/{device-name}`
- Exposes `GET /health` for liveness checks

Device discovery and command publishing live in the separate `mqtt-device-manager` project; this operator only ingests and stores telemetry.

</llm>


## :clipboard: Requirements

<llm requirements hint="Give the Terraform version from .terraform-version and the provider constraints from versions.tf, plus the credentials the providers need.">

- Go 1.26.2 (pinned in `go.mod`)
- GNU make
- Docker (or podman via `CONTAINER_TOOL`)
- A Kubernetes cluster (1.28+) with `kubectl` and a valid `kubeconfig`
- `kind` and a running Docker daemon (for `make test-e2e`)
- The `mqtt.hauke.cloud/v1alpha1` CRDs (from `github.com/hauke-cloud/kubernetes-iot-api v0.0.8`) installed in the cluster
- An MQTT broker (Tasmota) with a Kubernetes `Secret` containing `username` and `password`
- PostgreSQL or TimescaleDB (for `Database` CRs), with a password `Secret` or TLS client-cert/CA `Secret`s
- Helm 3 (for chart-based deployment)
- Terraform 1.9 (`.terraform-version`) and OpenTofu 1.8.0 (`.opentofu-version`); no Terraform or OpenTofu code is present in the repository

</llm>


## 🚀 Getting started

<llm getting_started hint="terraform init, plan and apply, with the backend configuration the repository actually uses. Say plainly if apply touches real infrastructure.">

1. Clone the repository and enter the project directory.

```bash
git clone https://github.com/hauke-cloud/mqtt-sensor-exporter.git
cd mqtt-sensor-exporter
```

2. Build the manager binary (requires Go ≥ 1.26.2 and GNU make).

```bash
make build
```

3. Run the operator against your current kubeconfig.

```bash
make run
```

The operator starts two reconcile loops (one for `MQTTBridge` CRs, one for `Database` CRs in the `mqtt.hauke.cloud/v1alpha1` group) and, by default, listens on `:8111` for the alerts REST API. Before it can store measurements you need the CRDs from the `kubernetes-iot-api` module installed in the cluster, a reachable MQTT broker with a credentials `Secret`, and a PostgreSQL or TimescaleDB instance referenced by a `Database` CR.

</llm>


## :airplane: Usage

<llm usage hint="For a reusable module, the central example is a module block with source, version and the required variables filled in from variables.tf. For a root module, show the workflow instead.">

Once the operator is running in your cluster, you configure it through custom resources and query it through the REST API.

**Deploy the operator**

Install the Helm chart (version `0.2.0`, default image `ghcr.io/hauke-cloud/mqtt-sensor-exporter`):

```bash
helm install mqtt-sensor-exporter ./deployments/helm/mqtt-sensor-exporter
```

Or build a single-file installer with kustomize and apply it:

```bash
make build-installer
kubectl apply -f dist/install.yaml
```

**Create the custom resources**

The operator watches three CR types under the group `mqtt.hauke.cloud/v1alpha1`. You create an `MQTTBridge` for your Tasmota bridge, a `Database` for your PostgreSQL/TimescaleDB instance, and one `Device` per sensor. The operator subscribes to the bridge topics, parses incoming `ZbReceived` payloads, matches them to `Device` CRs by Zigbee short address, and stores measurements in the database.

```yaml
apiVersion: mqtt.hauke.cloud/v1alpha1
kind: MQTTBridge
metadata:
  name: tasmota-bridge
  namespace: iot
spec:
  host: 192.168.1.50
  port: 1883
  deviceType: tasmota
  bridgeName: tasmota-bridge
  topics:
    - topic: tele/tasmota-bridge/SENSOR
      type: telemetry
      qos: 0
  credentialsSecretRef:
    name: mqtt-bridge-credentials
---
apiVersion: mqtt.hauke.cloud/v1alpha1
kind: Database
metadata:
  name: timescale
  namespace: iot
spec:
  host: timescale.iot.svc.cluster.local
  port: 5432
  database: iot
  username: iot_writer
  passwordSecretRef:
    name: db-password
  sslMode: require
  supportedSensorTypes:
    - moisture
    - water_level
    - valve
    - room
```

**Query the alerts API**

The REST API listens on `:8111` by default. You can evaluate alert conditions against stored measurements:

```bash
curl "http://<service>:8111/api/v2/alerts?location=garden&since=24h"
curl "http://<service>:8111/api/v2/alerts/moisture-sensor-01"
```

</llm>


## :wrench: Configuration

<llm configuration hint="A table of the variables in variables.tf: name, type, default, required. Point at variables.tf for the full set and mention outputs.tf if it exists.">

This repository contains no Terraform or OpenTofu files; the `.terraform-version` (1.9) and `.opentofu-version` (1.8.0) pins are template leftovers. Configuration is supplied through CLI flags, Helm values, and Custom Resource fields defined in the external module `github.com/hauke-cloud/kubernetes-iot-api v0.0.8`.

**CLI flags** (defined in `cmd/main.go`; see that file for the full set, including webhook and metrics certificate paths):

| Flag | Type | Default | Description |
|---|---|---|---|
| `--metrics-bind-address` | string | `"0"` (disabled) | Prometheus metrics endpoint address |
| `--health-probe-bind-address` | string | `:8081` | Health and readiness probe address |
| `--leader-elect` | bool | `false` | Enable leader election |
| `--api-bind-address` | string | `:8111` | REST alerts API address; set to `""` to disable |
| `--enable-http2` | bool | `false` | Enable HTTP/2 on the metrics server |

The only environment variable the binary reads is `POD_NAMESPACE` (fallback `default`).

**Helm values** (see `deployments/helm/mqtt-sensor-exporter/values.yaml` for the full set):

| Key | Type | Default | Description |
|---|---|---|---|
| `image.repository` | string | `ghcr.io/hauke-cloud/mqtt-sensor-exporter` | Container image |
| `operator.leaderElection` | bool | `true` | Enable leader election |
| `api.enabled` | bool | `true` | Start the REST alerts API |
| `api.port` | int | `8111` | REST API port |
| `crds.install` | bool | `true` | Install CRDs at startup |
| `logging.level` | string | `info` | Log level |
| `logging.format` | string | `json` | Log format |

The `MQTTBridge`, `Database`, and `Device` Custom Resources carry per-bridge, per-database, and per-sensor configuration (host, port, topics, credentials, sensor types, alert conditions). See `config/samples/` for example manifests and the `kubernetes-iot-api` module for the full schema.

</llm>


## :hammer: Development

<llm development hint="Cover terraform fmt, validate, tflint and terraform-docs where the repository configures them.">

Before pushing, run the same checks CI will:

```bash
make generate && make manifests
gofmt -s -l .
go vet ./...
go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
```

`make generate` and `make manifests` run controller-gen for DeepCopy methods and RBAC rules. If you change API types or RBAC annotations, commit the regenerated files or the pre-build step in `ci.yml` will fail.

For a stricter local lint pass (not enforced by CI but available):

```bash
make lint
```

This invokes golangci-lint v2.8.0 with a custom `sigs.k8s.io/logtools` logcheck plugin built from `.custom-gcl.yml`. Use `make lint-fix` to auto-fix what it can.

Unit tests use envtest (Kubernetes 1.35 assets):

```bash
make test
```

The e2e suite spins up a Kind cluster and runs Ginkgo:

```bash
make test-e2e
```

CI also enforces a conventional-commit PR title (`pr-title.yml`): the subject must start with `fix`, `feat`, `docs`, `ci`, or `chore` followed by a colon and a capital letter.

Pre-commit hooks (`.pre-commit-config.yaml`) check trailing whitespace, end-of-file newlines, AWS credentials, and gitleaks. Install them locally:

```bash
pre-commit install
pre-commit run --all-files
```

</llm>


## 📄 License

This Project is licensed under the GNU General Public License v3.0

- see the [LICENSE](LICENSE) file for details.


## :coffee: Contributing

To become a contributor, please check out the [CONTRIBUTING](CONTRIBUTING.md) file.


## :email: Contact

For any inquiries or support requests, please open an issue in this
repository or contact us at [contact@hauke.cloud](mailto:contact@hauke.cloud).
