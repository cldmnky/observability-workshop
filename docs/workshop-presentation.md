# OpenShift Observability Workshop
## Introduction & Architecture Overview

---

## Slide 1 — The Problem We're Solving

Your application is slow. Users are complaining. Where do you look?

```
Team A checks pod logs manually across 4 services...     → 45 minutes
Team B queries Prometheus but can't find the bottleneck  → 30 minutes
Team C opens a ticket asking for more information        → 2 hours

Mean Time to Resolution: 2–4 hours
```

**After this workshop:**

```
Alert fires → open trace → database span is 880ms → fix query → done

Mean Time to Resolution: 15–30 minutes
```

---

## Slide 2 — The Three Pillars

```
┌─────────────────────────────────────────────────────────────────────┐
│                    THE 3 PILLARS OF OBSERVABILITY                   │
├──────────────────┬──────────────────────┬───────────────────────────┤
│     METRICS      │        LOGS          │         TRACES            │
│                  │                      │                           │
│  Numeric trends  │  Timestamped events  │  End-to-end request flow  │
│  over time       │  with context        │  across services          │
│                  │                      │                           │
│  Rate            │  Error messages      │  frontend → backend       │
│  Errors          │  Transaction IDs     │    → database             │
│  Duration        │  User actions        │      → notifier           │
│                  │                      │                           │
│  → DETECT        │  → DIAGNOSE          │  → LOCATE                 │
└──────────────────┴──────────────────────┴───────────────────────────┘

Workflow:  metrics alert  →  log details  →  trace pinpoints the hop
```

---

## Slide 3 — The Demo Application

Four services. Plain HTTP. Intentionally simple so the focus stays on observability.

```
                           ┌──────────────────────────────────┐
                           │        USER BROWSER              │
                           └──────────────┬───────────────────┘
                                          │ HTTP
                                          ▼
                    ┌─────────────────────────────────────────┐
                    │           frontend  :8080  (Go)         │
                    │  SPA gateway │ source viewer │ /healthz  │
                    └──────────────────────┬──────────────────┘
                                           │ HTTP
                                           ▼
                    ┌─────────────────────────────────────────┐
                    │           backend   :8081  (Go)         │
                    │  Notes CRUD API │ random latency injected│
                    └──────────────┬───────────────┬──────────┘
                                   │               │
                          HTTP     ▼               ▼  HTTP
              ┌───────────────────────┐   ┌──────────────────────┐
              │  database  :8082 (Go) │   │ notifier :8083 (Py)  │
              │  SQLite + audit log   │   │ FastAPI event sink   │
              └───────────────────────┘   └──────────┬───────────┘
                                                     │ HTTP
                                                     ▼
                                          ┌──────────────────────┐
                                          │  database  :8082     │
                                          └──────────────────────┘
```

> Random delays of up to 60ms are injected in backend and database — making trace analysis realistic.

---

## Slide 4 — The Observability Stack

```
┌──────────────────────────────────────────────────────────────────────────┐
│                        OPENSHIFT CLUSTER                                 │
│                                                                          │
│  ┌───────────────── USER NAMESPACE ──────────────────────────────────┐  │
│  │                                                                    │  │
│  │   frontend ──┐                                                     │  │
│  │   backend  ──┼──► [otc-container sidecar]──► central-collector    │  │
│  │   database ──┤     (OTel Collector)           (observability-demo)│  │
│  │   notifier ──┘     pipes: traces+metrics+logs                     │  │
│  │                                                                    │  │
│  └────────────────────────────────────────────────────────────────────┘  │
│                                    │                                     │
│              ┌─────────────────────┼──────────────────────┐             │
│              │                     │                       │             │
│              ▼                     ▼                       ▼             │
│  ┌─────────────────┐  ┌──────────────────────┐  ┌──────────────────┐   │
│  │  TEMPO           │  │  COO PROMETHEUS       │  │  LOKISTACK       │   │
│  │  (traces)        │  │  (metrics +           │  │  (logs)          │   │
│  │  OTLP gRPC 4317  │  │   spanmetrics)        │  │  OTLP/HTTP 8080  │   │
│  │  openshift-tempo │  │  remote-write /write  │  │  openshift-      │   │
│  │  -operator       │  │  observability-demo   │  │  logging         │   │
│  └────────┬─────────┘  └──────────┬───────────┘  └────────┬─────────┘   │
│           │                       │                        │             │
│           └───────────────────────┼────────────────────────┘            │
│                                   │                                     │
│                                   ▼                                     │
│               ┌────────────────────────────────────────┐               │
│               │         OPENSHIFT CONSOLE              │               │
│               │  Observe → Traces  (Tempo UI)          │               │
│               │  Observe → Metrics (platform Prometheus)│               │
│               │  Observe → Logs    (LokiStack UI)      │               │
│               │  Observe → Dashboards (Perses + COO)   │               │
│               └────────────────────────────────────────┘               │
└──────────────────────────────────────────────────────────────────────────┘
```

---

## Slide 5 — OpenTelemetry Collector Architecture

Two-tier pipeline. Sidecar per pod → Central hub per namespace.

```
POD (your-namespace)                      observability-demo namespace
┌─────────────────────────────────┐      ┌──────────────────────────────────────┐
│  ┌────────────┐  ┌────────────┐ │      │  ┌──────────────────────────────┐   │
│  │ app        │  │otc-container│ │      │  │      central-collector        │   │
│  │ (Go SDK or │─►│  sidecar   │─┼─────►│  │                              │   │
│  │  py agent) │  │            │ │ OTLP │  │  receivers:  otlp (4317)     │   │
│  └────────────┘  │ pipelines: │ │      │  │                              │   │
│                  │  traces    │ │      │  │  connectors: spanmetrics ────┐│   │
│                  │  metrics   │ │      │  │                             ││   │
│                  │  logs      │ │      │  │  exporters:                 ││   │
│                  └────────────┘ │      │  │    otlp/tempo         ◄─────┘│   │
│                                 │      │  │    prometheusremotewrite     │   │
└─────────────────────────────────┘      │  │    prometheusremotewrite     │   │
                                         │  │    (spanmetrics pipeline)    │   │
                                         │  │    otlphttp/logs             │   │
                                         │  └──────────────────────────────┘   │
                                         └──────────────────────────────────────┘
```

> The `spanmetrics` connector is both an exporter (consumes traces) and a receiver (produces RED metrics) — zero application code required.

---

## Slide 5b — Built-in Stack vs. COO: Which One Do I Use?

OpenShift ships with three monitoring options. They are not mutually exclusive — all three can run simultaneously.

```
┌─────────────────────────┬──────────────────────────────┬──────────────────────────────────┐
│  PLATFORM MONITORING    │  USER WORKLOAD MONITORING    │  CLUSTER OBSERVABILITY           │
│  (CMO)                  │  (CMO)                       │  OPERATOR (COO)                  │
├─────────────────────────┼──────────────────────────────┼──────────────────────────────────┤
│ Namespace               │ Namespace                    │ Any namespace                    │
│ openshift-monitoring    │ openshift-user-workload-     │ (label-selector based)           │
│                         │ monitoring                   │                                  │
├─────────────────────────┼──────────────────────────────┼──────────────────────────────────┤
│ Cluster & control-plane │ Application metrics          │ Application metrics              │
│ health                  │ cluster-wide                 │ scoped per team / namespace      │
├─────────────────────────┼──────────────────────────────┼──────────────────────────────────┤
│ Managed by cluster      │ Configured by app teams      │ Configured by app teams          │
│ admins only             │ via ServiceMonitor           │ via MonitoringStack CR           │
├─────────────────────────┼──────────────────────────────┼──────────────────────────────────┤
│ Cannot be customised    │ Limited customisation        │ Full control: retention,         │
│                         │ (shared Prometheus)          │ replicas, resource limits        │
├─────────────────────────┼──────────────────────────────┼──────────────────────────────────┤
│ Observe → Metrics       │ Observe → Metrics            │ Observe → Dashboards (Perses)    │
│ (platform data)         │ (select your namespace)      │ COO Prometheus datasource        │
├─────────────────────────┼──────────────────────────────┼──────────────────────────────────┤
│ No custom dashboards    │ No custom dashboards         │ Perses dashboards built-in       │
│                         │                              │ Alertmanager per stack           │
├─────────────────────────┼──────────────────────────────┼──────────────────────────────────┤
│ ✗ Not for app metrics   │ ✓ Good for simple cases      │ ✓ Best for multi-tenant,         │
│                         │   single cluster             │   long-term, advanced use        │
└─────────────────────────┴──────────────────────────────┴──────────────────────────────────┘
```

**In this workshop we use both**:

```
Exercises 1–3  →  User Workload Monitoring (CMO)
                  ServiceMonitor, PromQL in Observe → Metrics

Exercise 4+    →  COO MonitoringStack
                  Perses dashboards, span-derived RED metrics,
                  independent Prometheus in observability-demo namespace
```

> **Important**: Span metrics (`traces_span_metrics_*`) land in COO Prometheus, not the platform Prometheus.
> You will not see them in **Observe → Metrics**. Use **Observe → Dashboards** with the COO datasource.

---

## Slide 6 — Module 1: User Workload Monitoring

**Duration**: 60 min full / 30 min abbreviated

**The ask**: Detect application issues *before* users report them.

```
OpenShift Monitoring Architecture
──────────────────────────────────────────────────────────────

openshift-monitoring          openshift-user-workload-monitoring
(CMO — platform)              (CMO — your apps)
  │ cluster health               │ ServiceMonitor → Prometheus
  │ infra metrics                │ PrometheusRule → Alertmanager
  │ READ ONLY                    │

observability-demo (COO)
  │ MonitoringStack CR
  │ Independent Prometheus + Alertmanager
  │ Label selector: monitoring.rhobs/stack=observability-stack
  │ Perses dashboards in Observe → Dashboards
```

**Exercises**:

| # | Exercise | What you build |
|---|---|---|
| 1 | Verify monitoring infrastructure | Confirm COO + UWM running |
| 2 | Deploy sample application | frontend/backend/database in your namespace |
| 3 | Configure ServiceMonitor | Prometheus scrapes app metrics |
| 4 | Write PromQL | RED method queries — rate, errors, p95 latency |
| 5 | Build Perses dashboard | `http_request_duration_seconds` heatmap + panels |
| 6 | Create PrometheusRule | Alert when error rate > 5% for 2 minutes |

**Key metric names**: `http_requests_total`, `http_request_duration_seconds`

---

## Slide 7 — Module 2: Logging with LokiStack

**Duration**: 60 min full / 30 min abbreviated

**The ask**: Stop SSH-ing into pods. Search logs across all services in seconds.

```
Log Flow
────────────────────────────────────────────────────────────────────

app stdout/stderr
      │
      ▼
container runtime  (node filesystem)
      │
      ▼
collector pods  (DaemonSet in openshift-logging)
      │  ClusterLogForwarder
      ▼
LokiStack  (openshift-logging)
  ├─ Distributor  ← receives streams
  ├─ Ingester     ← buffers + writes to ODF/S3
  ├─ Querier      ← LogQL queries
  └─ Compactor    ← index maintenance

      │  LogQL
      ▼
Observe → Logs (OpenShift console)
```

**Loki vs. Elasticsearch**: Loki indexes *only labels* (namespace, pod, container). Log content is stored unindexed and searched with regex at query time → much lower storage cost for Kubernetes workloads.

**Exercises**:

| # | Exercise | What you query |
|---|---|---|
| 1 | Verify logging infrastructure | `oc get pods -n openshift-logging` |
| 2 | Configure ClusterLogForwarder | Route app logs to LokiStack |
| 3 | Write LogQL queries | Filter by namespace, level, error keyword |
| 4 | Log-based alerting | Alert on error log pattern |
| 5 | Correlate logs + metrics | Find the log line behind a metric spike |

**Key LogQL pattern**: `{kubernetes_namespace_name="<ns>"} |= "error" | json`

---

## Slide 8 — Module 3: Distributed Tracing & OpenTelemetry

**Duration**: 75 min full / 30 min abbreviated

**The ask**: Find *which hop* is slow. Metrics say "p95 = 950ms". Traces say "database: 880ms".

```
A trace is a tree of spans
──────────────────────────────────────────────────────────────────

Trace: POST /api/notes  (134ms total)
  │
  ├── frontend: POST /api/notes         ████████████████████  134ms
  │     │
  │     └── frontend: HTTP POST         ███████████████████   130ms
  │           │
  │           ├── backend: POST /api/notes  ██████████████    125ms
  │           │     │
  │           │     ├── database: POST /notes  ████████████   115ms
  │           │     │
  │           │     └── notifier: POST /events  █████         40ms
  │           │             │
  │           │             └── database: POST /events  ████  32ms
```

**Context propagation**: `traceparent` HTTP header carries trace ID + span ID across every service boundary. W3C standard — works across Go, Python, Java, Node.js.

**Exercises**:

| # | Exercise | What you configure |
|---|---|---|
| 4 | Verify Tempo + sidecar collector | TempoStack CR, OpenTelemetryCollector sidecar CR |
| 5 | Create Instrumentation CR | Python zero-code injection config |
| 6 | Enable OTel on Go services | Annotations + env vars + service accounts |
| 7 | Explore live traces | TraceQL queries, waterfall, span attributes |
| 8 | Central collector pipeline | spanmetrics → Perses/COO, logs → Loki |
| 9 | Python auto-instrumentation | Zero-code notifier span appears in waterfall |

---

## Slide 9 — What Gets Instrumented and How

```
Service        Language   Method                        Who adds the spans?
──────────────────────────────────────────────────────────────────────────
frontend       Go         Manual SDK (telemetry.Setup)  Application code
backend        Go         Manual SDK (telemetry.Setup)  Application code
database       Go         Manual SDK (telemetry.Setup)  Application code
notifier       Python     Zero-code OTel Operator       Init-container injection

Sidecar adds to EVERY span (no app code):
  k8s.pod.name          ← k8sattributes processor
  k8s.deployment.name   ← k8sattributes processor
  k8s.namespace.name    ← k8sattributes processor
  cloud.platform        ← resourcedetection processor
  k8s.cluster.name      ← resourcedetection processor

Go SDK adds per service:
  http.method, http.status_code, http.route
  db.system, db.operation, db.sql.table
  peer.service, net.peer.name
  note.id, note.title (custom business attributes)
  baggage.client.platform, baggage.request.source (W3C Baggage)
```

---

## Slide 10 — Where Do I Look For What?

```
SIGNAL        WHERE TO LOOK                    DATASOURCE
──────────────────────────────────────────────────────────────────────────
Traces        Observe → Traces                 Tempo (shared, multi-tenant)
              TraceQL query language

App Metrics   Observe → Dashboards (Perses)    COO Prometheus
(spanmetrics  panel with "COO Prometheus" DS   (NOT platform Prometheus)
 RED metrics)

Platform      Observe → Metrics               Platform Prometheus
Metrics       PromQL

Logs          Observe → Logs                  LokiStack (openshift-logging)
              LogQL  {namespace} |= "error"

Dashboards    Observe → Dashboards            Perses (COO plugin)
              Perses + COO Prometheus DS

Alerts        Observe → Alerting              Alertmanager (UWM or COO)

⚠  span-derived RED metrics (traces_span_metrics_*) are in COO Prometheus.
   They are NOT visible in Observe → Metrics (platform Prometheus).
   Use Observe → Dashboards → Perses → COO datasource.
```

---

## Slide 11 — Software Versions

| Component | Version |
|---|---|
| OpenShift Container Platform | 4.21 |
| Cluster Observability Operator | 1.4.0 |
| Red Hat OpenShift Logging | 6.4.3 |
| Loki Operator | 6.4.3 |
| Red Hat build of OpenTelemetry | 0.144.0-1 |
| Tempo Operator | 0.20.0-2 |

All infrastructure pre-deployed via **ArgoCD GitOps** (ApplicationSet → Helm charts). Sync waves ensure operators install before CRs.

---

## Slide 12 — Expected Outcomes

```
BEFORE                              AFTER
──────────────────────────────────  ──────────────────────────────────────
SSH into pods to read logs          Observe → Logs, single LogQL query
"Which service is slow?" unknown    Trace waterfall shows 880ms in database
Manual metric correlation           Single Perses dashboard: metrics + spans
Alert from users                    PrometheusRule fires before users notice
MTTR: 2–4 hours                     MTTR: 15–30 minutes
```

---

*OpenShift Observability Workshop · OpenShift 4.21 · April 2026*
