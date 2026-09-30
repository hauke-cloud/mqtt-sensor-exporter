<!-- llm-readme-management spec=1 commit=36210983f23b11e1a8a0ba69f7c29b3f1924b4e1 template=terraform model=qwen3.8-27b-q4 digest=f68682b55584 generated=2026-09-30T14:02:40Z -->
<a href="https://hauke.cloud" target="_blank"><img src="https://img.shields.io/badge/home-hauke.cloud-brightgreen" alt="hauke.cloud" style="display: block;" /></a>
<a href="https://github.com/hauke-cloud" target="_blank"><img src="https://img.shields.io/badge/github-hauke.cloud-blue" alt="hauke.cloud Github Organisation" style="display: block;" /></a>
<a href="https://github.com/hauke-cloud/llm-readme-management" target="_blank"><img src="https://img.shields.io/badge/template-terraform-orange" alt="Repository type - terraform" style="display: block;" /></a>


# Template Repository


<img src="https://raw.githubusercontent.com/hauke-cloud/.github/main/resources/img/organisation-logo-small.png" alt="hauke.cloud logo" width="109" height="123" align="right">


<llm header hint="Say whether this is a reusable module or a root module that owns real state.">

This Go Kubernetes operator consumes Tasmota MQTT telemetry from Zigbee sensors and persists the measurements into PostgreSQL/TimescaleDB, with a REST API for querying alert conditions. It is a deployable service you install into a home or IoT Kubernetes cluster, then configure by declaring `MQTTBridge` and `Database` custom resources.

</llm>


## :book: Description

<llm description>

This is a Kubernetes operator that ingests Tasmota Zigbee sensor telemetry over MQTT and persists the measurements into PostgreSQL/TimescaleDB, with a REST endpoint for querying triggered alert conditions. If you run Tasmota-based Zigbee sensors on a Kubernetes cluster and need their readings in a queryable database with configurable alert thresholds, this is the component that connects the broker to the database.

It watches two custom resources — `MQTTBridge` and `Database` — whose types come from the `kubernetes-iot-api` module in the hauke-cloud organisation. For each bridge it opens an MQTT connection, subscribes to the configured sensor topics, parses Tasmota `ZbReceived` payloads, applies per-device additive corrections, and writes the result to the matching database. A companion HTTP API evaluates alert conditions declared on `Device` resources against the stored values.

- Connects to MQTT brokers (TCP or TLS) and subscribes to Tasmota `telemetry` and `sensor` topics
- Parses `ZbReceived` payloads and matches them to `Device` CRs by `status.shortAddr`
- Persists measurements for `moisture`, `water_level`, `valve`, and `room` sensor types
- Serves `GET /api/v2/alerts` with optional filters (`device-type`, `location`, `room`, `since`)
- Ships a Helm chart under `deployments/helm/mqtt-sensor-exporter/`

</llm>


## :clipboard: Requirements

<llm requirements hint="Give the Terraform version from .terraform-version and the provider constraints from versions.tf, plus the credentials the providers need.">

- Go 1.26 (go.mod pins 1.26.2)
- Docker, for building container images
- A Kubernetes cluster with the `mqtt.hauke.cloud/v1alpha1` CRDs (`mqttbridges`, `devices`, `databases`) already installed; this repository does not ship or generate CRDs
- `kubectl` configured against the target cluster
- `kind`, required by `make test-e2e`
- `helm`, for deploying via the chart in `deployments/helm/`
- `pre-commit`, per CONTRIBUTING.md
- A reachable MQTT broker with Tasmota devices publishing `ZbReceived` telemetry on the configured topics
- A PostgreSQL or TimescaleDB instance reachable from the cluster, with a role permitted to run the schema migrations from `database-iot-gorm`; credentials stored in Kubernetes Secrets
- An ingress controller (nginx or Traefik) providing TLS termination for the REST API; for the documented mTLS setup, a client-CA secret (e.g. `default/mqtt-api-client-ca`)

No cloud-provider credentials are required.

</llm>


## 🚀 Getting started

<llm getting_started hint="terraform init, plan and apply, with the backend configuration the repository actually uses. Say plainly if apply touches real infrastructure.">

1. Clone the repository.

```bash
git clone https://github.com/hauke-cloud/mqtt-sensor-exporter.git
cd mqtt-sensor-exporter
```

2. Build the operator binary.

```bash
make build
```

3. Run the operator against your cluster.

```bash
make run
```

The operator requires a Kubernetes cluster with the `mqtt.hauke.cloud/v1alpha1` CRDs (`mqttbridges`, `devices`, `databases`) already installed and `kubectl` configured for that cluster. This repository does not ship or generate those CRDs; they are provided by the external `kubernetes-iot-api` module. Once running, the operator watches for `MQTTBridge` and `Database` custom resources in the namespace set by `POD_NAMESPACE` (defaults to `default`), opens MQTT connections to the declared brokers, and begins persisting Tasmota sensor measurements into the configured PostgreSQL/TimescaleDB instances.

</llm>


## :airplane: Usage

<llm usage hint="For a reusable module, the central example is a module block with source, version and the required variables filled in from variables.tf. For a root module, show the workflow instead.">

Once the operator is running in your cluster, you interact with it by declaring custom resources and querying the REST API.

**Deploy the operator**

```bash
helm install mqtt-sensor-exporter ./deployments/helm/mqtt-sensor-exporter
```

The chart defaults to image `ghcr.io/hauke-cloud/mqtt-sensor-exporter:1.0.0`, one replica, and the REST API enabled on port `8111`.

**Declare an MQTTBridge and a Database**

Create a Kubernetes Secret with keys `username` and `password` for your broker, then apply an `MQTTBridge` CR:

```yaml
apiVersion: mqtt.hauke.cloud/v1alpha1
kind: MQTTBridge
metadata:
  name: tasmota-bridge
  namespace: mqtt-sensor-exporter-system
spec:
  host: mqtt.example.com
  port: 1883
  deviceType: tasmota
  credentialsSecretRef:
    name: mqtt-credentials
  topics:
    - topic: "tele/+/SENSOR"
      type: telemetry
      qos: 0
```

Similarly, create a Secret with key `password` for the database and apply a `Database` CR:

```yaml
apiVersion: mqtt.hauke.cloud/v1alpha1
kind: Database
metadata:
  name: timescale
  namespace: mqtt-sensor-exporter-system
spec:
  host: db.example.com
  port: 5432
  database: iot
  username: exporter
  passwordSecretRef:
    name: db-credentials
  sslMode: require
  supportedSensorTypes:
    - moisture
    - room
```

The operator connects to the broker, subscribes to the listed topics, and writes matched Tasmota `ZbReceived` payloads into the database. Devices must already exist as `Device` CRs (matched by `status.shortAddr`); the operator does not create or update them.

**Query alerts**

The REST API serves plain HTTP (authentication is expected at the ingress):

```bash
curl "http://<service>:8111/api/v2/alerts?since=5m&room=kitchen"
```

Supported query parameters: `device-type`, `location`, `room`, `since` (e.g. `5m`). When `since` is set, alert conditions are evaluated against the average over that window; otherwise the latest single measurement is used.

</llm>


## :wrench: Configuration

<llm configuration hint="A table of the variables in variables.tf: name, type, default, required. Point at variables.tf for the full set and mention outputs.tf if it exists.">

This repository is not a Terraform module; it contains no `.tf` files. Configuration is supplied through CLI flags, one environment variable, and Helm chart values.

Helm values (full set in `deployments/helm/mqtt-sensor-exporter/values.yaml`):

| Name | Type | Default | Description |
|------|------|---------|-------------|
| `image.repository` | string | `ghcr.io/hauke-cloud/mqtt-sensor-exporter` | Container image |
| `image.tag` | string | `1.0.0` | Image tag (defaults to `appVersion`) |
| `replicaCount` | int | `1` | Operator pod replicas |
| `operator.leaderElection` | bool | `true` | Enable leader election |
| `operator.metrics.enabled` | bool | `true` | Enable metrics endpoint |
| `operator.metrics.port` | int | `8080` | Metrics port |
| `operator.health.port` | int | `8081` | Health-probe port |
| `api.enabled` | bool | `true` | Enable the REST alert API |
| `api.port` | int | `8111` | API listen port |
| `api.bindAddress` | string | `":8111"` | API bind address |
| `api.service.type` | string | `ClusterIP` | Kubernetes Service type for the API |

CLI flags (`cmd/main.go`): `--metrics-bind-address` (default `"0"`, i.e. disabled), `--health-probe-bind-address` (`:8081`), `--leader-elect` (`false`), `--metrics-secure` (`true`), `--api-bind-address` (`:8111`), `--enable-http2` (`false`), plus standard `zap` logging flags.

Environment variable: `POD_NAMESPACE` — namespace used by the `MQTTBridge` and `Database` watchers; falls back to `default` when unset.

The operator also reads `MQTTBridge`, `Database`, and `Device` custom-resource specs at runtime (types from `github.com/hauke-cloud/kubernetes-iot-api`); see `config/samples/` for example manifests.

</llm>


## :hammer: Development

<llm development hint="Cover terraform fmt, validate, tflint and terraform-docs where the repository configures them.">

Before pushing, install the pre-commit hooks and run them across the tree:

```bash
pre-commit install
pre-commit run --all-files
```

CI also enforces a conventional-commit title on every PR: the type must be `fix`, `feat`, `docs`, `ci`, or `chore`, and the subject must start with an uppercase letter.

Run the unit tests (envtest-backed) and the e2e suite (requires `kind`):

```bash
make test
make test-e2e
```

CI executes `go test -v -race -coverprofile=coverage.out -covermode=atomic ./...` on every push and pull request.

Format and lint before pushing:

```bash
make fmt
make vet
make lint
```

CI checks `gofmt -s -l .` and `go vet ./...`. The linter is golangci-lint v2 built with a custom `logcheck` plugin (see `.custom-gcl.yml`).

Two generated-file targets must be run and their output committed, or CI will fail on a diff:

```bash
make generate
make manifests
```

Both invoke controller-gen for DeepCopy methods and RBAC manifests. This repository does not generate CRDs; the API types come from the external `kubernetes-iot-api` module.

</llm>


## 📄 License

This Project is licensed under the GNU General Public License v3.0

- see the [LICENSE](LICENSE) file for details.


## :coffee: Contributing

To become a contributor, please check out the [CONTRIBUTING](CONTRIBUTING.md) file.


## :email: Contact

For any inquiries or support requests, please open an issue in this
repository or contact us at [contact@hauke.cloud](mailto:contact@hauke.cloud).
