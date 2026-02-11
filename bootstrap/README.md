# OpenShift Observability Workshop - Bootstrap

This directory contains ArgoCD ApplicationSet and Helm charts to bootstrap the complete observability stack for the workshop.

## Chart Categories

### Production Charts (from actual repo manifests)

Based on [observability-developer-day-2025](https://github.com/cldmnky/observability-developer-day-2025) production-ready manifests:

- **coo-monitoring-stack**: Cluster Observability Operator with namespace-scoped MonitoringStack (Prometheus, Alertmanager, ThanosQuerier)
- **tempo-stack**: TempoStack with MinIO S3 storage and multi-tenancy (dev/prod tenants)
- **opentelemetry-stack**: Auto-instrumentation with sidecar + central collector architecture and spanmetrics for RED metrics
- **observability-servicemonitors**: ServiceMonitors for demo applications with automatic discovery

### Workshop Charts (educational supplements)

Additional charts for workshop modules (commented out by default in ApplicationSet):

- **user-workload-monitoring**: Enables cluster-wide user workload monitoring (Module 1 alternative to COO)
- **logging-operator**: OpenShift Logging with LokiStack for centralized logging (Module 2)

**Note**: Workshop charts are disabled by default since they weren't in the production implementation. Uncomment them in the ApplicationSet if you want to run the full 4-module workshop.

## Overview

### Production Stack Architecture

```
Applications (Go, Python, Quarkus, Node.js)
    ├─→ Metrics: ServiceMonitors → Prometheus (COO MonitoringStack)
    └─→ Traces: Auto-instrumentation → Sidecar Collector → Central Collector
                                                              ├─→ Tempo (with X-Scope-OrgID)
                                                              └─→ Spanmetrics → Prometheus (RED metrics)
```

## Prerequisites

- OpenShift 4.21+ cluster with cluster-admin access
- OpenShift GitOps (ArgoCD) operator installed
- **S3 Object Storage** for Tempo traces and Loki logs (choose one):
  - **Option A (Recommended): OpenShift Data Foundation (ODF)** - Automatic bucket provisioning via ObjectBucketClaim
    - ODF operator installed (`openshift-storage namespace`)
    - NooBaa/MCG enabled
    - StorageClass: `openshift-storage.noobaa.io`
  - **Option B: External S3** (MinIO, AWS S3, etc.)
    - Endpoint accessible from cluster
    - Bucket created
    - Access credentials (access_key_id and access_key_secret)

## S3 Storage Configuration

### Using ODF (Recommended)

If you have OpenShift Data Foundation installed, the charts will automatically create S3 buckets using ObjectBucketClaims:

**Check if ODF is available:**

```bash
# Check for NooBaa system
oc get noobaa -n openshift-storage

# Check for ODF StorageClass
oc get storageclass openshift-storage.noobaa.io
```

**Configure charts to use ODF:**

Both `tempo-stack` and `logging-operator` charts default to ODF (`storage.type: odf` in values.yaml). No additional configuration needed!

**ObjectBucketClaim automatically creates:**
- S3 bucket with unique name
- Secret with S3 credentials (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)
- ConfigMap with bucket details (BUCKET_NAME, BUCKET_HOST, BUCKET_PORT)

### Using External S3

If using external S3 (MinIO, AWS S3, etc.), update chart values:

**For tempo-stack:**
```yaml
storage:
  type: external
  external:
    secretName: tempostack-dev-minio
    endpoint: storage.example.com:9199
    bucket: tempo-bucket
    accessKeyId: "<your-key>"
    accessKeySecret: "<your-secret>"
```

**For logging-operator:**
```yaml
lokiStack:
  s3:
    type: external
    external:
      endpoint: "https://storage.example.com:9199"
      region: "us-east-1"
      bucketnames: "loki-bucket"
      accessKeyId: "<your-key>"
      accessKeySecret: "<your-secret>"
```

## Deployment

### Option 1: GitOps with ArgoCD (Recommended)

1. **Update repository URL** in the ApplicationSet:

Edit `bootstrap/argocd/applicationset-observability.yaml` and replace the `repoURL` with your fork:

```yaml
repoURL: https://github.com/<your-org>/observability-workshop.git
```

2. **(Optional) Configure external S3 storage**:

If using **external S3** instead of ODF, edit `bootstrap/helm/tempo-stack/values.yaml`:

```yaml
storage:
  type: external  # Change from "odf" to "external"
  external:
    secretName: tempostack-dev-minio
    endpoint: storage.example.com:9199
    bucket: tempo-bucket
    accessKeyId: "<your-access-key-id>"
    accessKeySecret: "<your-secret-access-key>"
```

For **ODF (default)**, no configuration needed - buckets are provisioned automatically!

3. **Apply the ApplicationSet**:

```bash
oc apply -f bootstrap/argocd/applicationset-observability.yaml
```

4. **Monitor sync status**:

```bash
oc get applications -n openshift-gitops
```

5. **Verify deployments**:

```bash
# COO MonitoringStack
oc get monitoringstack -n observability-demo
oc get pods -n observability-demo -l app.kubernetes.io/name=prometheus

# Tempo TempoStack
oc get tempostack -n openshift-tempo-operator
oc get pods -n openshift-tempo-operator -l app.kubernetes.io/component=distributor

# OpenTelemetry Collectors
oc get opentelemetrycollector -n observability-demo
oc get pods -n observability-demo -l app.kubernetes.io/name=central-collector-collector

# ServiceMonitors
oc get servicemonitor -n observability-demo -l monitoring.rhobs/stack=observability-stack
```

### Enable Workshop Charts (Optional)

To run the full 4-module workshop including Module 2 (Logging) and the alternative Module 1 approach:

1. **Edit the ApplicationSet** to uncomment workshop charts:

```bash
vi bootstrap/argocd/applicationset-observability.yaml
```

Uncomment these lines:

```yaml
# User Workload Monitoring (Workshop Module 1 - Alternative to COO)
- name: user-workload-monitoring
  namespace: openshift-monitoring
  repoURL: https://github.com/cldmnky/observability-workshop.git
  targetRevision: main
  path: bootstrap/helm/user-workload-monitoring

# Logging Operator with LokiStack (Workshop Module 2 - Not in production)
- name: logging-operator
  namespace: openshift-logging
  repoURL: https://github.com/cldmnky/observability-workshop.git
  targetRevision: main
  path: bootstrap/helm/logging-operator
```

2. **Apply the updated ApplicationSet**:

```bash
oc apply -f bootstrap/argocd/applicationset-observability.yaml
```

3. **Verify workshop components**:

```bash
# User Workload Monitoring
oc get pods -n openshift-user-workload-monitoring

# Logging with LokiStack
oc get pods -n openshift-logging
oc get lokistack -n openshift-logging
```

**Production vs Workshop Trade-offs:**

| Approach | Pros | Cons | Use For |
|----------|------|------|---------|
| **COO MonitoringStack** | Namespace-scoped, label-based discovery, modern | Requires COO operator | Production deployments |
| **User Workload Monitoring** | Built-in, cluster-wide, simple setup | No namespace isolation | Workshops, simple clusters |
| **Logging with LokiStack** | Full log aggregation, powerful queries | Not in production repo, resource-intensive | Workshop Module 2 |

### Option 2: Manual Deployment with Helm
```

**Wait for operator to be ready:**

```bash
oc wait --for=condition=Ready csv -l operators.coreos.com/cluster-observability-operator.openshift-cluster-observability-operator \
  -n openshift-cluster-observability-operator --timeout=300s
```

#### 2. Deploy Tempo TempoStack

**IMPORTANT**: Update S3 credentials in `values.yaml` first!

```bash
helm upgrade --install tempo-stack \
  ./bootstrap/helm/tempo-stack \
  --namespace openshift-tempo-operator \
  --create-namespace \
  --set storage.accessKeyId="<your-access-key>" \
  --set storage.accessKeySecret="<your-secret-key>"
```

**Wait for operator to be ready:**

```bash
oc wait --for=condition=Ready csv -l operators.coreos.com/tempo-product.openshift-tempo-operator \
  -n openshift-tempo-operator --timeout=300s
```

#### 3. Deploy OpenTelemetry Stack

```bash
helm upgrade --install opentelemetry-stack \
  ./bootstrap/helm/opentelemetry-stack \
  --namespace observability-demo \
  --create-namespace
```

**Wait for operator to be ready:**

```bash
oc wait --for=condition=Ready csv -l operators.coreos.com/opentelemetry-product.openshift-operators \
  -n openshift-operators --timeout=300s
```

#### 4. Deploy ServiceMonitors

```bash
helm upgrade --install observability-servicemonitors \
  ./bootstrap/helm/observability-servicemonitors \
  --namespace observability-demo
```

## Helm Charts

### 1. coo-monitoring-stack

**Purpose**: Installs Cluster Observability Operator and creates namespace-scoped MonitoringStack.

**Key Configuration** (`values.yaml`):

```yaml
monitoringStack:
  namespace: observability-demo
  name: observability-stack
  resourceSelector:
    matchLabels:
      monitoring.rhobs/stack: observability-stack  # ServiceMonitors with this label auto-discovered
  retention: 1d
  prometheus:
    replicas: 3
  alertmanager:
    replicas: 2
thanosQuerier:
  enabled: true  # Multi-tenancy query aggregation
```

**Resources Created**:
- Namespace: `openshift-cluster-observability-operator`
- OperatorGroup, Subscription for COO
- Namespace: `observability-demo`
- MonitoringStack CR: Prometheus (3 replicas), Alertmanager (2 replicas)
- ThanosQuerier CR: Query aggregation layer

### 2. tempo-stack

**Purpose**: Installs Tempo Operator and creates TempoStack with S3 storage (ODF or external) and multi-tenancy.

**Key Configuration** (`values.yaml`):

```yaml
storage:
  type: odf  # "odf" (default) or "external"
  
  # ODF storage (automated bucket provisioning via ObjectBucketClaim)
  odf:
    enabled: true
    bucketName: tempo-bucket
    storageClassName: openshift-storage.noobaa.io
    reclaimPolicy: Delete
  
  # External S3 (manual bucket + credentials)
  external:
    secretName: tempostack-dev-minio
    endpoint: storage.example.com:9199
    bucket: tempo-bucket
    accessKeyId: ""  # REQUIRED if using external
    accessKeySecret: ""  # REQUIRED if using external

tempoStack:
  size: 1x.small  # 1x.extra-small, 1x.small, 1x.medium, 1x.large
  retention:
    global: 48h
  replicationFactor: 1  # 3 for production
  gateway:
    enabled: true  # OpenShift authentication
  jaegerUI:
    enabled: true  # Jaeger UI via query-frontend

tenants:
  dev:
    id: 1610b0c3-c509-4592-a256-a1871353dbfa  # Used in X-Scope-OrgID header
  prod:
    id: 6094b0f1-711d-4395-82c0-30c2720c6648
```

**Resources Created**:
- Namespace: `openshift-tempo-operator`
- OperatorGroup, Subscription for Tempo
- **If ODF**: ObjectBucketClaim `tempo-bucket` (auto-creates Secret + ConfigMap)
- **If external**: Secret `tempostack-dev-minio` with S3 credentials
- RBAC: ClusterRole `tempostack-traces-reader`
- TempoStack CR: distributor, ingester, querier, query-frontend, compactor, gateway
- Route: `tempo-tempo-gateway` for Jaeger UI access

**S3 Storage Verification**:

If using ODF:
```bash
# Check ObjectBucketClaim status
oc get objectbucketclaim tempo-bucket -n openshift-tempo-operator

# View auto-created credentials
oc get secret tempo-bucket -n openshift-tempo-operator -o yaml

# View bucket metadata
oc get configmap tempo-bucket -n openshift-tempo-operator -o yaml
```

### 3. opentelemetry-stack

**Purpose**: Installs OpenTelemetry Operator with auto-instrumentation, sidecar collectors, and central collector with spanmetrics.

**Key Configuration** (`values.yaml`):

```yaml
instrumentation:
  name: demo-instrumentation
  exporter:
    endpoint: http://localhost:4318  # Sidecar collector
  sampler:
    type: parentbased_traceidratio
    argument: "1.0"  # 100% sampling for demo

centralCollector:
  replicas: 2
  tempo:
    endpoint: tempo-tempo-distributor.openshift-tempo-operator.svc:4317
    tenant: dev  # X-Scope-OrgID header value
  prometheus:
    endpoint: http://observability-stack-prometheus.observability-demo.svc:9090/api/v1/write
  spanmetrics:
    enabled: true  # Generates RED metrics from traces
    exemplarsEnabled: true  # Links metrics to traces
```

**Resources Created**:
- Subscription for OpenTelemetry Operator (openshift-operators ns)
- ServiceAccounts: `otel-collector-sidecar`, `otel-central-collector`
- RBAC: ClusterRole for k8sattributes processor, Tempo write permissions
- Instrumentation CR: Auto-instrumentation for Python/Node.js/Java/Go
- OpenTelemetryCollector CR (sidecar mode): Injected into pods via annotation
- OpenTelemetryCollector CR (deployment mode): Central collector with spanmetrics connector

**Enable auto-instrumentation**:

Add annotations to your deployment:

```yaml
spec:
  template:
    metadata:
      annotations:
        sidecar.opentelemetry.io/inject: "sidecar"
        instrumentation.opentelemetry.io/inject-python: "demo-instrumentation"  # Or nodejs, java, go
    spec:
      serviceAccountName: otel-collector-sidecar
```

### 4. observability-servicemonitors

**Purpose**: Creates ServiceMonitors for demo applications with automatic discovery by MonitoringStack.

**Key Configuration** (`values.yaml`):

```yaml
namespace: observability-demo
monitoringLabel:
  key: monitoring.rhobs/stack
  value: observability-stack  # Matches MonitoringStack resourceSelector
scrapeInterval: 30s

serviceMonitors:
  goApi:
    enabled: true
    serviceName: go-api
    portNumber: 8080
  pythonApi:
    enabled: true
    serviceName: python-api
    portNumber: 8000
  # ... etc
```

**Resources Created**:
- ServiceMonitor: `go-api` (port 8080/metrics)
- ServiceMonitor: `python-api` (port 8000/metrics)
- ServiceMonitor: `quarkus-api` (port 4003/metrics)
- ServiceMonitor: `node-app` (port 3000/metrics)

All ServiceMonitors have label `monitoring.rhobs/stack: observability-stack` for automatic discovery.

### 5. user-workload-monitoring (Workshop Chart)

**Purpose**: Enables cluster-wide user workload monitoring (Workshop Module 1 alternative).

**Note**: This is an alternative to COO MonitoringStack. Use COO for production, this for workshops/simple setups.

**Key Configuration** (`values.yaml`):

```yaml
monitoring:
  enabled: true
  prometheusUserWorkload:
    enabled: true
    retention: 24h
    volumeClaimTemplate:
      spec:
        storageClassName: gp3-csi
        resources:
          requests:
            storage: 10Gi
    resources:
      requests:
        cpu: 100m
        memory: 512Mi
  thanosQuerier:
    enabled: true
```

**Resources Created**:
- ConfigMap: `cluster-monitoring-config` in `openshift-monitoring` (enables user workload monitoring)
- ConfigMap: `user-workload-monitoring-config` in `openshift-user-workload-monitoring`
- Prometheus pods in `openshift-user-workload-monitoring` namespace
- Thanos Querier for query aggregation

**Differences from COO**:
- Cluster-wide (all namespaces) vs namespace-scoped
- ConfigMap-based vs CR-based management
- Built-in OpenShift feature vs external operator
- Simpler but less flexible

### 6. logging-operator (Workshop Chart)

**Purpose**: OpenShift Logging with LokiStack for centralized logging (Workshop Module 2).

**Note**: This was NOT in the production implementation but needed for workshop Module 2.

**Key Configuration** (`values.yaml`):

```yaml
operator:
  namespace: openshift-logging
  channel: stable-6.2

lokiStack:
  enabled: true
  name: logging-loki
  size: 1x.extra-small
  storageClassName: gp3-csi
  storage:
    size: 10Gi
  tenantMode: openshift-logging
  
  # S3 storage for logs (ODF, external, or PVC)
  s3:
    type: odf  # "odf" (default), "external", or "pvc"
    
    # ODF storage (automated bucket provisioning)
    odf:
      enabled: true
      bucketName: loki-bucket
      storageClassName: openshift-storage.noobaa.io
      reclaimPolicy: Delete
    
    # External S3 (manual bucket + credentials)
    external:
      endpoint: "https://storage.example.com:9199"
      region: "us-east-1"
      bucketnames: "loki-bucket"
      accessKeyId: ""  # REQUIRED if using external
      accessKeySecret: ""  # REQUIRED if using external

clusterLogging:
  enabled: true
  collection:
    logs:
      application:
        enabled: true  # Collect app logs
      infrastructure:
        enabled: true  # Collect OpenShift logs
      audit:
        enabled: false # Skip audit logs (resource-intensive)
```

**Resources Created**:
- Namespace: `openshift-logging`
- OperatorGroup, Subscription for Cluster Logging Operator
- **If ODF**: ObjectBucketClaim `loki-bucket` (auto-creates Secret + ConfigMap)
- **If external**: Secret `logging-loki-s3` with S3 credentials
- **If PVC**: Uses PersistentVolumeClaims (no S3)
- LokiStack CR: Loki components (compactor, distributor, ingester, querier, query-frontend)
- ClusterLogging CR: Vector log collector (runs on all nodes)

**S3 Storage Verification**:

If using ODF:
```bash
# Check ObjectBucketClaim status
oc get objectbucketclaim loki-bucket -n openshift-logging

# View auto-created credentials
oc get secret loki-bucket -n openshift-logging -o yaml

# View bucket metadata
oc get configmap loki-bucket -n openshift-logging -o yaml
```

**Access Logs**:
- OpenShift Console: Observe → Logs
- LogQL query language (similar to PromQL for logs)

## Customization

### Adjust Resource Limits

Edit `values.yaml` in each Helm chart:

**COO MonitoringStack**:

```yaml
monitoringStack:
  prometheus:
    replicas: 5  # Scale up
    resources:
      requests:
        cpu: 200m
        memory: 512Mi
      limits:
        cpu: 1
        memory: 1Gi
```

**Tempo TempoStack**:

```yaml
tempoStack:
  size: 1x.medium  # Scale up from 1x.small
  resources:
    total:
      limits:
        cpu: 4  # Increase from 2
        memory: 4Gi  # Increase from 2Gi
  retention:
    global: 168h  # 7 days (increase from 48h)
```

**OpenTelemetry Central Collector**:

```yaml
centralCollector:
  replicas: 4  # Scale up from 2
  memoryLimit: 3600  # Increase from 1800 MiB
```

### Change S3 Storage Backend

Update `bootstrap/helm/tempo-stack/values.yaml`:

```yaml
storage:
  endpoint: my-minio.example.com:9000
  bucket: my-tempo-bucket
  # Update credentials via --set or sealed-secrets
```

### Configure Additional Tenants

Update `bootstrap/helm/tempo-stack/values.yaml`:

```yaml
tenants:
  staging:
    id: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
    authentication:
      type: static
```

Update `bootstrap/helm/tempo-stack/templates/tempostack.yaml` to include new tenant.

### Adjust Spanmetrics Histogram Buckets

Update `bootstrap/helm/opentelemetry-stack/values.yaml`:

```yaml
centralCollector:
  spanmetrics:
    histogramBuckets:
      - 1ms
      - 5ms
      - 10ms
      - 50ms
      - 100ms
      - 500ms
      - 1s
      - 2s
      - 5s
      - 10s
      - 30s
```

## Verification

### Check MonitoringStack Status

```bash
# MonitoringStack CR status
oc get monitoringstack observability-stack -n observability-demo -o jsonpath='{.status.conditions}'

# Prometheus pods
oc get pods -n observability-demo -l app.kubernetes.io/name=prometheus

# Access Prometheus UI
oc port-forward -n observability-demo svc/observability-stack-prometheus 9090:9090
# Open: http://localhost:9090
```

### Check TempoStack Status

```bash
# TempoStack CR status
oc get tempostack tempo -n openshift-tempo-operator -o jsonpath='{.status.conditions}'

# Tempo components
oc get pods -n openshift-tempo-operator

# Access Jaeger UI
JAEGER_URL=$(oc get route tempo-tempo-gateway -n openshift-tempo-operator -o jsonpath='{.spec.host}')
echo "Jaeger UI: https://$JAEGER_URL/api/traces/v1/dev/search"
```

### Check OpenTelemetry Collectors

```bash
# Instrumentation CR
oc get instrumentation demo-instrumentation -n observability-demo

# OpenTelemetryCollector CRs
oc get opentelemetrycollector -n observability-demo

# Central collector pods
oc get pods -n observability-demo -l app.kubernetes.io/name=central-collector-collector

# Check logs
oc logs -n observability-demo -l app.kubernetes.io/name=central-collector-collector -c otc-container
```

### Query RED Metrics from Spanmetrics

```bash
# Port-forward to Prometheus
oc port-forward -n observability-demo svc/observability-stack-prometheus 9090:9090
```

Open http://localhost:9090 and query:

```promql
# Request rate (calls/sec)
sum by (service_name) (rate(traces_spanmetrics_calls_total[5m]))

# Error rate
sum by (service_name) (rate(traces_spanmetrics_calls_total{http_status_code=~"5.."}[5m]))

# P95 latency
histogram_quantile(0.95, sum by (service_name, le) (rate(traces_spanmetrics_latency_bucket[5m])))
```

## Troubleshooting

### Operator Installation Failures

**Check CSV status**:

```bash
# COO
oc get csv -n openshift-cluster-observability-operator

# Tempo
oc get csv -n openshift-tempo-operator

# OpenTelemetry
oc get csv -n openshift-operators | grep opentelemetry
```

**Check operator pod logs**:

```bash
oc logs -n openshift-cluster-observability-operator -l app.kubernetes.io/name=cluster-observability-operator
oc logs -n openshift-tempo-operator -l app.kubernetes.io/name=tempo-operator
oc logs -n openshift-operators -l app.kubernetes.io/name=opentelemetry-operator
```

### MonitoringStack Not Creating Prometheus

**Check MonitoringStack status**:

```bash
oc describe monitoringstack observability-stack -n observability-demo
```

**Common issues**:
- Insufficient RBAC: COO operator needs cluster-admin or similar permissions
- Resource limits: Ensure cluster has enough resources for 3 Prometheus replicas

### TempoStack Not Starting

**Check TempoStack status**:

```bash
oc describe tempostack tempo -n openshift-tempo-operator
```

**Common issues**:
- S3 secret missing or invalid: Verify secret `tempostack-dev-minio` exists with correct fields
- S3 connectivity: Check if endpoint is reachable from cluster
- Invalid tenant IDs: Ensure tenant IDs are valid UUIDs

**Test S3 connectivity**:

```bash
oc run test-s3 --image=amazon/aws-cli --rm -it --restart=Never -- \
  s3 ls --endpoint-url https://storage.blahonga.me:9199 s3://borg-tempo
```

### ObjectBucketClaim Not Creating S3 Bucket

**Check ObjectBucketClaim status**:

```bash
# For Tempo
oc get objectbucketclaim tempo-bucket -n openshift-tempo-operator
oc describe objectbucketclaim tempo-bucket -n openshift-tempo-operator

# For Loki
oc get objectbucketclaim loki-bucket -n openshift-logging
oc describe objectbucketclaim loki-bucket -n openshift-logging
```

**Check ODF operator status**:

```bash
# Check ODF CSV
oc get csv -n openshift-storage | grep odf-operator

# Check NooBaa status (should be "Ready")
oc get noobaa noobaa -n openshift-storage -o jsonpath='{.status.phase}'

# Check NooBaa S3 endpoints
oc get noobaa noobaa -n openshift-storage -o jsonpath='{.status.services.serviceS3}'
```

**Verify StorageClass exists**:

```bash
oc get storageclass openshift-storage.noobaa.io
```

**Check OBC events for errors**:

```bash
oc get events -n openshift-tempo-operator --field-selector involvedObject.name=tempo-bucket
```

**Common issues**:
- ODF not installed: Run `oc get csv -n openshift-storage` to verify
- NooBaa not ready: Check `oc get pods -n openshift-storage` for failing pods
- StorageClass missing: ODF may not be configured for MCG/NooBaa
- Resource quota exceeded: Check namespace ResourceQuota limits

### TempoStack/LokiStack Can't Find S3 Secret

**Check if secret was created by OBC**:

```bash
# For Tempo (secret name matches bucketName)
oc get secret tempo-bucket -n openshift-tempo-operator

# For Loki
oc get secret loki-bucket -n openshift-logging
```

**Verify secret has required keys**:

```bash
# Should show: AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, BUCKET_HOST, BUCKET_NAME, BUCKET_PORT
oc get secret tempo-bucket -n openshift-tempo-operator -o jsonpath='{.data}' | jq 'keys'
```

**Check TempoStack references correct secret**:

```bash
oc get tempostack tempo -n openshift-tempo-operator -o yaml | grep -A5 storage:
# Should see:
#   storage:
#     secret:
#       name: tempo-bucket  # Matches OBC name
```

**Recreate ObjectBucketClaim if secret is missing**:

```bash
# Delete and recreate to trigger secret generation
oc delete objectbucketclaim tempo-bucket -n openshift-tempo-operator
helm upgrade tempo-stack ./bootstrap/helm/tempo-stack -n openshift-tempo-operator
```

### OpenTelemetry Sidecar Not Injected

**Check Instrumentation CR**:

```bash
oc get instrumentation demo-instrumentation -n observability-demo
```

**Verify pod annotations**:

```bash
oc get pod <pod-name> -n observability-demo -o jsonpath='{.metadata.annotations}'
```

**Check for sidecar container**:

```bash
oc get pod <pod-name> -n observability-demo -o jsonpath='{.spec.containers[*].name}'
# Should show: <app-container> otc-container
```

**Common issues**:
- Missing annotation: `sidecar.opentelemetry.io/inject: "sidecar"`
- Missing instrumentation annotation: `instrumentation.opentelemetry.io/inject-<lang>: "demo-instrumentation"`
- Wrong service account: Needs `serviceAccountName: otel-collector-sidecar`
- Operator not ready: Check OpenTelemetry operator logs

### Traces Not Appearing in Jaeger

**Check central collector logs**:

```bash
oc logs -n observability-demo -l app.kubernetes.io/name=central-collector-collector -c otc-container | grep -i tempo
```

**Verify Tempo write permissions**:

```bash
oc describe clusterrole tempo-traces-writer
oc describe clusterrolebinding otel-central-collector-tempo-writer
```

**Check X-Scope-OrgID header**:

```bash
# Should see header in central-collector config
oc get opentelemetrycollector central-collector -n observability-demo -o yaml | grep -A5 headers
```

**Test trace query**:

```bash
# Get Jaeger UI URL
JAEGER_URL=$(oc get route tempo-tempo-gateway -n openshift-tempo-operator -o jsonpath='{.spec.host}')

# Query traces via API
curl -H "Authorization: Bearer $(oc whoami -t)" \
  "https://$JAEGER_URL/api/traces/v1/dev/search?service=<service-name>"
```

### ServiceMonitors Not Discovered

**Check ServiceMonitor labels**:

```bash
oc get servicemonitor -n observability-demo --show-labels
```

**Ensure label matches MonitoringStack resourceSelector**:

```bash
# Should show: monitoring.rhobs/stack=observability-stack
oc get servicemonitor -n observability-demo -l monitoring.rhobs/stack=observability-stack
```

**Check Prometheus targets**:

```bash
# Port-forward to Prometheus
oc port-forward -n observability-demo svc/observability-stack-prometheus 9090:9090

# Open: http://localhost:9090/targets
# ServiceMonitors should appear as targets
```

## ArgoCD Sync Policy

The ApplicationSet uses automated sync with:

- **Prune**: Removes resources deleted from Git
- **Self-heal**: Automatically reverts manual changes
- **Retry**: Retries failed syncs with exponential backoff (5 attempts, 5s to 3m)

To disable automated sync:

```yaml
syncPolicy:
  automated: null  # Remove this section for manual sync
```

To adjust retry policy:

```yaml
syncPolicy:
  retry:
    limit: 10  # Increase retry attempts
    backoff:
      duration: 10s
      factor: 2
      maxDuration: 5m
```

## Multi-Tenancy

Tempo is configured with two tenants by default:

- **dev** (ID: `1610b0c3-c509-4592-a256-a1871353dbfa`)
- **prod** (ID: `6094b0f1-711d-4395-82c0-30c2720c6648`)

The central OpenTelemetry collector sends traces to the **dev** tenant by default using the `X-Scope-OrgID` header.

To send traces to a different tenant, update `bootstrap/helm/opentelemetry-stack/values.yaml`:

```yaml
centralCollector:
  tempo:
    tenant: prod  # Change from dev to prod
```

## RED Metrics (Spanmetrics)

The spanmetrics connector in the central OpenTelemetry collector automatically generates **RED metrics** (Rate, Errors, Duration) from traces:

- `traces_spanmetrics_calls_total`: Request count
- `traces_spanmetrics_latency`: Latency histogram (with exemplars)
```

Should contain `enableUserWorkload: true`.

## Directory Structure

```
bootstrap/
├── argocd/
│   └── applicationset-observability.yaml    # ArgoCD ApplicationSet
└── helm/
    ├── user-workload-monitoring/
    │   ├── Chart.yaml


Metrics are automatically sent to Prometheus and include exemplars that link back to the originating trace.

## Directory Structure

```
bootstrap/
├── argocd/
│   └── applicationset-observability.yaml  # ArgoCD ApplicationSet for GitOps deployment
└── helm/
    ├── coo-monitoring-stack/              # Cluster Observability Operator + MonitoringStack
    │   ├── Chart.yaml
    │   ├── values.yaml
    │   └── templates/
    │       ├── coo-namespace.yaml
    │       ├── coo-operatorgroup.yaml
    │       ├── coo-subscription.yaml
    │       ├── monitoring-namespace.yaml
    │       ├── monitoring-stack.yaml
    │       ├── thanos-querier.yaml
    │       └── NOTES.txt
    ├── tempo-stack/                       # Tempo Operator + TempoStack with S3 storage
    │   ├── Chart.yaml
    │   ├── values.yaml
    │   └── templates/
    │       ├── tempo-namespace.yaml
    │       ├── tempo-operatorgroup.yaml
    │       ├── tempo-subscription.yaml
    │       ├── tempo-secret.yaml
    │       ├── tempo-rbac.yaml
    │       ├── tempostack.yaml
    │       └── NOTES.txt
    ├── opentelemetry-stack/               # OpenTelemetry Operator + collectors + auto-instrumentation
    │   ├── Chart.yaml
    │   ├── values.yaml
    │   └── templates/
    │       ├── operator-subscription.yaml
    │       ├── rbac.yaml
    │       ├── tempo-writer-rbac.yaml
    │       ├── instrumentation.yaml
    │       ├── sidecar-collector.yaml
    │       ├── central-collector.yaml
    │       └── NOTES.txt
    └── observability-servicemonitors/     # ServiceMonitors for demo applications
        ├── Chart.yaml
        ├── values.yaml
        └── templates/
            ├── go-api-servicemonitor.yaml
            ├── python-api-servicemonitor.yaml
            ├── quarkus-api-servicemonitor.yaml
            ├── node-app-servicemonitor.yaml
            └── NOTES.txt
```

## References

- [OpenShift GitOps Documentation](https://docs.openshift.com/gitops/latest/)
- [Cluster Observability Operator](https://docs.openshift.com/container-platform/latest/observability/monitoring/monitoring-overview.html)
- [Red Hat build of Tempo](https://docs.openshift.com/container-platform/latest/observability/distr_tracing/distr_tracing_tempo/distr-tracing-tempo-installing.html)
- [Red Hat build of OpenTelemetry](https://docs.openshift.com/container-platform/latest/observability/otel/otel-installing.html)
- [Helm Documentation](https://helm.sh/docs/)

## Support

For issues or questions:

1. Check the [Troubleshooting](#troubleshooting) section above
2. Review operator logs for errors
3. Verify all prerequisites are met (especially S3 storage for Tempo)
4. Consult the official Red Hat documentation linked above

## Contributing

To contribute improvements to these Helm charts:

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Test deployment on a real OpenShift cluster
5. Submit a pull request with detailed description

---

**Last Updated**: Based on observability-developer-day-2025 manifests structure
