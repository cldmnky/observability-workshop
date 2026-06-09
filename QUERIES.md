# OpenShift Observability Workshop — Query Reference Guide (user1)

This document is a comprehensive reference of **PromQL (Prometheus)**, **LogQL (Loki)**, and **TraceQL (Tempo)** queries designed for **`user1`** in the **`user1-observability-demo`** namespace. It covers all three pillars of observability across both legacy scraping pipelines and native OpenTelemetry SDK / connector pathways.

---

## 🏗️ Observability Architecture & Pipelines

When querying signals, understanding the pathway of each signal is critical to choosing the correct datasource and labels:

```text
SIGNAL           SOURCE                           PATHWAY                                                   DATASOURCE (Perses / Console)
──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────
Metrics (Scraped) Application /metrics             ServiceMonitor Scrape ⇢ COO Prometheus                    COO Prometheus
Metrics (OTel)    Application OTel SDK            Sidecar ⇢ Central Collector ⇢ COO Prometheus              COO Prometheus (remote-write)
Metrics (Spans)   Trace Spans                     OTel Connector ⇢ COO Prometheus                           COO Prometheus (remote-write)

Logs (stdout)     stdout (JSON access logs)       Vector (DaemonSet) ⇢ LokiStack (app tenant)               LokiStack (application)
Logs (OTel)       slog.InfoContext (OTel bridge)  Sidecar ⇢ Central Collector ⇢ LokiStack                   LokiStack (application)

Traces (OTel)     Application OTel SDK            Sidecar ⇢ Central Collector ⇢ Tempo (dev tenant)          Tempo (dev)
```

---

## 📈 1. PromQL Queries (COO Prometheus / Perses)

All metrics below are stored in the **COO Prometheus** instance. In Perses dashboards, select the **COO Prometheus** datasource.

[IMPORTANT]
====
*Scraped metrics* (ServiceMonitor → Prometheus scrape) carry the built-in `namespace` label.
*Remote-written OTel metrics* (sidecar → central-collector → `prometheusremotewrite`) carry the `k8s_namespace_name` label (converted from the `k8s.namespace.name` resource attribute by `resource_to_telemetry_conversion`).

Mixing `namespace=` with OTel metrics or `k8s_namespace_name=` with scraped metrics returns no data.
====

### A. ServiceMonitor Scraped Metrics (Module 1 / CMO-compatible)
These metrics are collected by traditional Prometheus scraping of the `/metrics` endpoint. They use the `prom_` prefix and are recorded by the Go application's custom access-log middleware.

#### 1. HTTP Request Rate per Second (by Route)
Calculates the rate of incoming requests over a 5-minute sliding window, grouped by HTTP route and method:
```promql
sum by (method, route, service) (
  rate(prom_http_requests_total{namespace="user1-observability-demo"}[5m])
)
```

#### 2. HTTP Error Rate Percentage (by Service & Route)
Calculates the percentage of HTTP status codes matching 4xx or 5xx over a 5-minute window:
```promql
sum by (route, service) (
  rate(prom_http_requests_total{namespace="user1-observability-demo", status=~"[45].."}[5m])
)
/
sum by (route, service) (
  rate(prom_http_requests_total{namespace="user1-observability-demo"}[5m])
) * 100
```

#### 3. p95 Request Latency (by Route)
Calculates the 95th percentile request latency using the histogram buckets scraped from the metrics handler:
```promql
histogram_quantile(0.95,
  sum by (le, route) (
    rate(prom_http_request_duration_seconds_bucket{namespace="user1-observability-demo"}[5m])
  )
)
```

---

### B. Native OpenTelemetry SDK Metrics (Module 3 / OTel SDK)
These metrics are generated natively by the OpenTelemetry Go SDK inside our microservices and pushed over OTLP. They bypass HTTP scraping entirely.

#### 1. Frontend Proxied Requests Rate (by Target Method, Path & Backend Status)
Tracks requests forwarded from the `frontend` proxy to the downstream `backend` API. The frontend explicitly labels the downstream HTTP status as `backend_status` to distinguish it from the frontend's own response status:
```promql
sum by (method, path, backend_status) (
  rate(frontend_requests_proxied_total{k8s_namespace_name="user1-observability-demo"}[5m])
)
```

#### 2. Backend Request Processing Rate (by Route & HTTP Status)
Tracks requests processed at the core business API layer inside the `backend` deployment:
```promql
sum by (route, method, http_status) (
  rate(backend_requests_processed_total{k8s_namespace_name="user1-observability-demo"}[5m])
)
```

#### 3. Notifier Notification Rate (by Action)
Counts the rate of lifecycle notification requests (note created, updated, or deleted) sent to the Python `notifier` microservice:
```promql
sum by (action) (
  rate(backend_notifications_sent_total{k8s_namespace_name="user1-observability-demo"}[5m])
)
```

#### 4. Database Write Rates (Events vs. Notes)
Compares direct database write transaction counters inside the `database` service:
```promql
# Rate of database audit events recorded
rate(database_events_created{k8s_namespace_name="user1-observability-demo"}[5m])

# Rate of user notes created
rate(database_notes_created{k8s_namespace_name="user1-observability-demo"}[5m])
```

---

### C. Trace-Derived RED Metrics (Spanmetrics Connector)
These metrics are generated by the OTel collector's `spanmetrics` connector in `observability-demo`. It analyzes trace spans in real time, generates call/error rates and latencies, and remote-writes them to Prometheus.

#### 1. Trace-Derived Call Volume Rate (by Service, Span Kind & Span Name)
Provides call rates for any span (HTTP endpoints, database queries, notifier calls) completely independent of code-level metric libraries. The spanmetrics connector exposes `span_kind` as a built-in dimension (`SPAN_KIND_SERVER`, `SPAN_KIND_CLIENT`, `SPAN_KIND_INTERNAL`, etc.):
```promql
sum by (service_name, span_kind, span_name) (
  rate(traces_spanmetrics_calls_total{k8s_namespace_name="user1-observability-demo"}[5m])
)
```

#### 2. Trace-Derived Error Rate Percentage
Isolates errors in traces where the span status was flagged as `STATUS_CODE_ERROR`:
```promql
sum by (service_name) (
  rate(traces_spanmetrics_calls_total{k8s_namespace_name="user1-observability-demo", status_code="STATUS_CODE_ERROR"}[5m])
)
/
sum by (service_name) (
  rate(traces_spanmetrics_calls_total{k8s_namespace_name="user1-observability-demo"}[5m])
) * 100
```

#### 3. Trace-Derived p99 Latency (by Service and Operation)
Calculates 99th percentile execution time of specific tracing spans (highly useful for database query timing):
```promql
histogram_quantile(0.99,
  sum by (le, service_name, span_name) (
    rate(traces_spanmetrics_latency_bucket{k8s_namespace_name="user1-observability-demo"}[5m])
  )
)
```

#### 4. Error Rate by HTTP Method (spanmetrics custom dimensions)
The workshop config adds `http.method` and `http.status_code` as custom spanmetrics dimensions. Filter for 5xx errors on POST requests:
```promql
sum by (http_method) (
  rate(traces_spanmetrics_calls_total{k8s_namespace_name="user1-observability-demo", http_method="POST", http_status_code=~"5.."}[5m])
)
```

---

## 🪵 2. LogQL Queries (Loki / Log Browser)

Loki is accessible in the console via **Observe** → **Logs**. The URL path is `/monitoring/logs`.

### A. Standard stdout Access Logs (JSON via Vector)
Access logs are printed as raw JSON strings to standard output by the `AccessLog` middleware, bypassed by the OTel logging bridge. The log structure is **nested double-JSON**: the first layer is the container runtime/Vector envelope (`hostname`, `kubernetes`, `message`, `openshift`), and the actual access fields (`method`, `route`, `status`, etc.) live inside the `message` field as a **JSON string**.

[IMPORTANT]
====
`| json message` extracts the `message` field as a label from the outer JSON — it does **NOT** re-parse its content as JSON. To access fields inside `message`, you must use a pipeline that re-writes the log line:

```
| json                        # extracts all outer fields (kubernetes_namespace_name, message, level, …)
| line_format "{{.message}}"  # replaces the log line with just the message string
| json                        # parses the message as fresh JSON → extracts method, route, status, …
```

Alternatively, use `| json msg="message"` to store the raw `message` string, then `| line_format "{{.msg}}" | json` to avoid clobbering the `message` label.
====

#### 1. Query All Standard Access Logs for the Namespace
Fetches all stdout application logs and parses the nested `message` field into available labels (`method`, `route`, `status`, `duration_ms`, `service`):
```logql
{kubernetes_namespace_name="user1-observability-demo"} |= "access" | json | line_format "{{.message}}" | json
```

#### 2. Isolate Slow Access Logs (> 50ms)
Filters parsed access logs where the processing duration exceeds 50 milliseconds:
```logql
{kubernetes_namespace_name="user1-observability-demo"} |= "access" | json | line_format "{{.message}}" | json | duration_ms > 50
```

#### 3. Count Requests by Service and Status Code
Generates a time-series rate chart of HTTP responses grouped by HTTP status:
```logql
sum by (service, status) (
  count_over_time({kubernetes_namespace_name="user1-observability-demo"} |= "access" | json | line_format "{{.message}}" | json [5m])
)
```

#### 4. Filter by HTTP Method and Route
Finds PUT or DELETE write transactions targeting notes:
```logql
{kubernetes_namespace_name="user1-observability-demo"} |= "access" | json | line_format "{{.message}}" | json | method =~ "PUT|DELETE" | route =~ "/api/notes.*"
```

---

### B. OTel-Bridged Application Logs (OTLP via Central Collector)
These are application-level logs emitted via `slog.InfoContext` and bridged through the OTel SDK (`otelslog` handler). They travel through the sidecar → central collector → LokiStack (OTLP/HTTP) pipeline. **Unlike the stdout access logs, these are natively parsed by Loki:** the log body IS the raw log line, and OTel attributes become Loki *structured metadata* — no `| json` parser needed.

#### 1. Find All Application OTel Logs (by Body Substring)
Since the OTel log body is the raw message string, use `|=` (line filter):
```logql
{kubernetes_namespace_name="user1-observability-demo"} |= "proxied"
```

#### 2. Filter Logs by Specific Trace ID (Log-to-Trace Correlation)
When Loki ingests OTLP logs natively, the trace/span IDs become structured metadata keys `trace_id` (lowercase, underscore) — not camelCase:
```logql
{kubernetes_namespace_name="user1-observability-demo"} | trace_id = "4bf92f3577b34da6a3ce929d0e0e4736"
```

#### 3. Find Database Write Audit Logs
Filters OTel logs where the body contains the database service's write audit message:
```logql
{kubernetes_namespace_name="user1-observability-demo"} |= "event created"
```

#### 4. Find All Note Creation Activity
```logql
{kubernetes_namespace_name="user1-observability-demo"} |= "note created"
```

#### 5. Find Large Content Note Creation
OTel log attributes become structured metadata with dots replaced by underscores (e.g., `note.content_length` → `note_content_length`):
```logql
{kubernetes_namespace_name="user1-observability-demo"} |= "note created" | note_content_length > 100
```

#### 6. Query Backend Database Proxy Logs with Errors
Structured metadata supports numeric comparisons via Loki auto-coercion:
```logql
{kubernetes_namespace_name="user1-observability-demo"} |= "proxied to database" | http_status >= 400
```

#### 7. Filter by Specific OTel Attribute Value (e.g., Route)
```logql
{kubernetes_namespace_name="user1-observability-demo"} |= "proxied" | route = "/api/notes"
```

#### 8. Find All ERROR Severity OTel Logs
The OTel severity is stored as `severity_text` structured metadata:
```logql
{kubernetes_namespace_name="user1-observability-demo"} | severity_text = "ERROR"
```

---

## 🕸️ 3. TraceQL Queries (Tempo / Traces UI)

Tempo is accessible via **Observe** → **Traces** using the **Loki/Tempo** multi-tenant gateway. Select the **`openshift-tempo-operator/tempo`** instance and **`dev`** tenant.

### A. Basic Trace Hunting
Finds traces based on standard service and span characteristics.

#### 1. Find Slow Traces across Any Service
Finds any end-to-end trace that took longer than 500 milliseconds:
```traceql
{ duration > 500ms }
```

#### 2. Find Traces with Errors
Finds traces containing at least one span marked with an error status:
```traceql
{ status = error }
```

#### 3. Filter Traces by Root Service and Endpoint
Finds traces where the transaction started as a POST request to `/api/notes` at the `frontend` gateway. In TraceQL, `resource.service.name` scopes to resource-level attributes and `name` is an intrinsic span name field:
```traceql
{ resource.service.name = "frontend" && name = "POST /api/notes" }
```

---

### B. Attribute-Based Trace Hunting
Leverages custom span attributes injected by our manual SDK instrumentation and OTel sidecars.

#### 1. Find SQL Database Transactions (Chainsql)
Isolates database operations on the SQLite backend. Use `span.` scope for span-level custom attributes (replaces the deprecated `attribute.` prefix in modern Tempo 2.4+):
```traceql
{ span.db.system = "chainsql" && span.db.operation = "SELECT" }
```

#### 2. Find Specific Database Table Writes
Finds write transactions targeting the `notes` table specifically:
```traceql
{ span.db.sql.table = "notes" && span.db.operation = "INSERT" }
```

#### 3. Filter Traces by Custom Baggage (Workshop-Specific)
Filters traces initiated from a specific client platform (W3C Baggage propagated automatically across all hops). Baggage members are recorded as span attributes with the `baggage.` prefix:
```traceql
{ span.baggage.client.platform = "web" && span.baggage.request.source = "workshop-demo" }
```

#### 4. Search Traces by Business Attribute (Note Title)
Finds the exact trace corresponding to a note titled "Audit Log":
```traceql
{ span.note.title = "Audit Log" }
```

#### 5. Search Python Notifier Traces (ASGI / httpx)
Finds notifier microservice spans with HTTP status 200:
```traceql
{ resource.service.name = "notifier" && span.http.status_code = 200 }
```

---

### C. Structural & Relationship Queries
Advanced queries matching child-parent execution flows and performance regressions.

#### 1. Slow Downstream Database Proxy (Backend calling slow Database write)
Finds traces where the `backend` called the `database` service, and the database write itself took longer than 80ms:
```traceql
{ resource.service.name = "backend" } >> { resource.service.name = "database" && duration > 80ms }
```

#### 2. Slow Python Notifier processing
Finds traces where a fast API frontend call triggered a slow async background notification in the Python `notifier` service:
```traceql
{ resource.service.name = "frontend" && duration < 200ms } >> { resource.service.name = "notifier" && duration > 100ms }
```

#### 3. Database Write Spans holding up HTTP responses
Finds transactions where the root frontend span is slow (> 150ms) because it is waiting on a blocking database insert:
```traceql
{ resource.service.name = "frontend" && duration > 150ms } >> { span.db.operation = "INSERT" && duration > 50ms }
```
