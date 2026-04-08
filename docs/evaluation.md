# Module 1: User Workload Monitoring — Lab Evaluation

Evaluated by running through all exercises against cluster `apps.cluster-xwmfw.dynamic.redhatworkshops.io` as `user3` on April 8, 2026.

---

## Summary

All 5 exercises completed successfully. The core flow works well and the lab clearly communicates the difference between CMO-managed user workload monitoring and the Cluster Observability Operator. Several actionable improvements were found.

---

## Screenshots captured

| File | Exercise | Notes |
| --- | --- | --- |
| `user-workload-monitoring-console.png` | Ex 1 | Pods view in `openshift-user-workload-monitoring` — all 5 pods Running |
| `user-workload-metrics.png` | Ex 1 | `up{namespace="user3-observability-demo"}` query — both sample-app pod targets `Value: 1` |
| `sample-app-pods.png` | Ex 2 | Both sample-app pods `2/2 Running` in `user3-observability-demo` |
| `user-workload-queries.png` | Ex 3 | `rate(http_requests_total...)` by HTTP status code (200/404/500) with real data |
| `observability-stack.png` | Ex 4 | Topology view of COO stack in `observability-demo` (Prometheus StatefulSet, Thanos Querier, Alertmanager, plus OpenTelemetry central-collector) |
| `perses-dashboards.png` | Ex 4 | **Live** Perses dashboard showing HTTP Request Rate, Error Rate %, and Duration panels with real data |
| `alert-rules.png` | Ex 5 | Alerting rules page — `LowRequestRate` **Firing** after clearing Platform source filter |

---

## Findings & Improvements

### 1. `PersesDatasource` API version mismatch (Bug — HIGH)

**Found in**: Exercise 4 step 5.

The lab guide shows:

```yaml
apiVersion: perses.dev/v1alpha1
kind: PersesDatasource
```

But the cluster has deployed `perses.dev/v1alpha2` and the datasource YAML uses `v1alpha2`. Applying the YAML from step 6 (the `PersesDashboard`) produces:

```text
Warning: perses.dev/v1alpha1 is deprecated; use perses.dev/v1alpha2
```

**Fix**: Update all `apiVersion: perses.dev/v1alpha1` references to `perses.dev/v1alpha2` in the lab guide and in the example YAML blocks for both `PersesDatasource` and `PersesDashboard`.

---

### 2. `PersesDatasource` expected YAML doesn't match actual (Bug — MEDIUM)

**Found in**: Exercise 4 step 5.

The lab guide shows the expected output with `apiVersion: perses.dev/v1alpha1`, but the actual deployed resource uses `v1alpha2`. This will confuse learners comparing output.

Additionally, the actual resource has extra fields (ArgoCD annotations, `status.conditions`) that the guide omits, which is fine — but the `apiVersion` discrepancy will cause confusion.

**Fix**: Update the `.Expected output` block to show `v1alpha2` and remove or note the additional ArgoCD annotations.

---

### 3. Exercise 4 step 4: `oc exec` into `observability-demo` fails for regular users (Bug — HIGH)

**Found in**: Exercise 4 step 4.

The guide instructs:

```bash
oc exec -n observability-demo prometheus-observability-stack-0 -c prometheus -- \
  curl -s http://localhost:9090/api/v1/targets | ...
```

Regular workshop users (`user3`) cannot exec into pods in `observability-demo`:

```text
Error from server (Forbidden): pods "prometheus-observability-stack-0" is forbidden:
User "user3" cannot create resource "pods/exec" in API group "" in the namespace "observability-demo"
```

**Fix option A**: Remove or replace this verification step with a PromQL query via the Perses UI or via the console metrics page scoped to `observability-demo`.

**Fix option B**: Grant `pods/exec` in `observability-demo` to workshop users (security trade-off).

**Recommended fix**: Replace the exec-based verification with querying Prometheus via its in-cluster service using a `curl` from a pod the user *does* own:

```bash
# Query COO Prometheus from sample-app debug sidecar
oc exec -n user3-observability-demo deployment/sample-app -c debug -- \
  curl -s 'http://observability-stack-prometheus.observability-demo.svc.cluster.local:9090/api/v1/targets' | \
  grep 'sample-app-coo'
```

---

### 4. Exercise 1: PromQL query `up{namespace="openshift-user-workload-monitoring"}` returns "Access restricted" (Bug — MEDIUM)

**Found in**: Exercise 1 step 5.

When navigating to **Observe → Metrics** with "All Projects" context and running:

```promql
up{namespace="openshift-user-workload-monitoring"}
```

The result is `"Access restricted. Bad Request"`.

Regular users cannot query across `openshift-user-workload-monitoring` from the admin metrics page. They need to use the developer metrics view or a query scoped to their own namespace.

**Fix**: Update the Exercise 1 PromQL example to use the user's own namespace or use a query that works cluster-wide for the user:

```promql
up{namespace="user3-observability-demo"}
```

Or instruct users to navigate to **Observe → Metrics** while in the `openshift-user-workload-monitoring` project context (use the project switcher first).

---

### 5. Exercise 1 step 3: Navigation instruction doesn't match UI (Content — LOW)

**Found in**: Exercise 1 step 3.

The guide says: *"Navigate to **Observe** → **Metrics** in the left navigation"* and *"make sure the namespace filter is set to `openshift-user-workload-monitoring`"*.

In the current OCP 4.21 console, the "Metrics" page for non-admin users automatically resolves to project-scoped metrics. There is no "namespace filter" dropdown on the query browser — project context is set via the Project dropdown at the top.

**Fix**: Clarify that users should first switch the **Project** dropdown (top of page) to `openshift-user-workload-monitoring` before navigating to **Observe** → **Metrics**.

---

### 6. Exercise 5: `LowRequestRate` alert hidden by default Platform source filter (Bug — MEDIUM)

**Found in**: Exercise 5.

After creating the `PrometheusRule`, navigating to **Observe → Alerting → Alerting rules** shows an empty list. The alert does not appear because the Alerting rules tab defaults to the **Source: Platform** filter.

Clicking **Alerting rules** navigates to a URL with `?rowFilter-alerting-rule-source=platform`, which filters out user-defined rules created via `PrometheusRule` in the user's own namespace. Clicking **(X)** to remove the "Platform" chip — or clicking **Clear all filters** — immediately reveals the `LowRequestRate` rule.

**Root cause**: The default filter ("Platform") hides rules with `Source: User`. This is not an RBAC issue — user3 has the correct permissions. It is a UX issue with the console's default filter selection.

**Fix**: Update the lab guide to instruct users to clear the Platform source filter (or select Source: User) to see their alert rules. Add a screenshot showing the alert visible after clearing the filter.

**Status**: Fixed in lab guide — Exercise 5 step 3 now explains the Platform filter and includes a screenshot of `LowRequestRate` in Firing state.

---

### 7. COO topology screenshot description misleading (Content — LOW)

**Found in**: Exercise 4 step 2.

The lab guide says:
> "You may also notice the `opentelemetry` application containing the `central-collector` deployment"

In the UI, this is displayed as a separate application *above* the `observability-stack` group, labeled `APPL opentelemetry`. It is easy to confuse visually. The screenshot shows both but the guide text only mentions the OpenTelemetry app in parentheses.

**Fix**: Update the guide text to mention both application groups visible in the topology: `opentelemetry` (with `central-collector`) and `observability-stack` (with Prometheus, Alertmanager, Thanos Querier).

---

### 8. Screenshots need update

The following pre-existing screenshots in `assets/images/` need replacement with the freshly captured ones:

| Image | Status |
| --- | --- |
| `user-workload-monitoring-console.png` | ✅ Updated — shows real pods |
| `user-workload-metrics.png` | ✅ Updated — shows `up{namespace="user3-observability-demo"}` with both sample-app pods `Value: 1` |
| `user-workload-queries.png` | ✅ Updated — shows `rate(http_requests_total...)` by HTTP status code with real data |
| `observability-stack.png` | ✅ Updated — live topology view |
| `perses-dashboards.png` | ✅ Updated — live dashboard with real traffic data |
| `alert-rules.png` | ✅ Updated — shows `LowRequestRate` in Firing state after clearing Platform source filter |

**All screenshots are now captured and current.** Previously "missing" screenshots have been taken this session:

- `user-workload-metrics.png` — retaken showing `up{namespace="user3-observability-demo"}` query with both pods `Value: 1`
- `user-workload-queries.png` — retaken showing `rate(http_requests_total...)` by HTTP status code
- `alert-rules.png` — retaken showing `LowRequestRate` Firing with Warning severity
- `sample-app-pods.png` — added to Exercise 2 in the lab guide

---

---

## Module 2: Logging with LokiStack — Lab Evaluation

Evaluated by running through all exercises against cluster `apps.cluster-xwmfw.dynamic.redhatworkshops.io` as `user3` on April 8, 2026.

> **Note**: Screenshots for Module 2 (Logs) are not included in this evaluation — they are not required as part of the evaluation criteria.

---

## Summary

All 5 exercises completed successfully. The LokiStack is fully operational and log-based alerting is functional. Three actionable bugs were found — two are command/URL accuracy issues (broken copy-paste commands) and one is a significant discrepancy between what the lab guide claims and what OpenShift 4.21 actually shows in the Alerting UI for Loki-based rules.

---

## Exercises Completed

| Exercise | Description | Status |
|---|---|---|
| Ex1 | App deployed (frontend/backend/database/loadgenerator/notifier all Running) | ✅ |
| Ex2 | LokiStack verified — all component pods Running in `openshift-logging` | ✅ |
| Ex3 | LogQL queries executed, logs scoped to namespace | ✅ |
| Ex4 | LogQL aggregation/rate queries run | ✅ |
| Ex5 | `AlertingRule` (Loki) created, confirmed firing in Alertmanager | ✅ |

---

## Findings

### M2-1: ClusterLogForwarder name mismatch (Bug — MEDIUM)

**Found in**: Exercise 2, verification step.

The lab guide instructs:

```bash
oc get clusterlogforwarder -n openshift-logging instance
```

**Actual result**:

```text
Error from server (NotFound): clusterlogforwarders.observability.openshift.io "instance" not found
```

The deployed `ClusterLogForwarder` is named `logging-collector`, not `instance`.

**Before**:
```bash
oc get clusterlogforwarder -n openshift-logging instance
```

**After**:
```bash
oc get clusterlogforwarder -n openshift-logging logging-collector
```

**Fix**: Update all CLF verification commands in Exercise 2 to use `logging-collector` as the resource name.

---

### M2-2: Logs UI URL incorrect (Bug — MEDIUM)

**Found in**: Exercise 3, navigation step.

The lab guide directs users to navigate to a URL that results in a 404 error:

```
/k8s/ns/user3-observability-demo/observe/logs
```

**Actual working URL**:

```
/monitoring/logs
```

The OpenShift 4.21 console uses `/monitoring/logs` for the Log Browser, not the path shown in the guide.

**Before**:
> "Navigate to Observe → Logs in the console"
> *(links to `/k8s/ns/user3-observability-demo/observe/logs`)*

**After**:
> "Navigate to **Observe → Logs** in the left navigation. The URL path should be `/monitoring/logs`."

**Fix**: Update the URL reference and navigation instructions in all Exercise 3 steps that reference the Logs page link.

---

### M2-3: Loki AlertingRule not visible in OCP console Alerting page (Bug — HIGH)

**Found in**: Exercise 5, final verification step.

The lab guide states:

> "Navigate to **Observe → Alerting → Alerting rules** to see your log-based alert alongside metric-based ones."

**Actual behavior**: The `Observe → Alerting → Alerting rules` page in the OCP console **exclusively queries the Prometheus rules API** (`/api/v1/rules`). It does **not** display `AlertingRule` CRs (`loki.grafana.com/v1`) evaluated by the Loki ruler.

The alert IS firing (confirmed via Alertmanager API and `oc get alertingrule`), but it does **not appear** on the Alerting rules page in the console alongside the PrometheusRule-based alerts. There is no current UX path in the OCP console UI to list Loki-based alerting rules.

**Evidence**:
```bash
# Alert IS in Alertmanager
oc get alertingrule -n user3-observability-demo
# NAME               AGE
# error-rate-alert   Xm

# Alert IS in Alertmanager (firing state confirmed via API)
oc exec -n openshift-logging alertmanager-logging-0 -- \
  wget -qO- http://localhost:9093/api/v2/alerts | jq '.[] | select(.labels.alertname=="HighErrorRate")'
```

**Impact on learners**: Medium-to-high. Learners follow the guide step-by-step and will find the Alerting rules page empty after creating the `AlertingRule`. They may assume the exercise failed when in fact the alert is working correctly but just not surfaced in the UI.

**Before**:
> "Navigate to Observe → Alerting → Alerting rules to verify the alert appears."

**After**:
> "Loki-based `AlertingRule` CRs are evaluated by the Loki ruler and appear in Alertmanager, but are **not currently listed** on the OCP console Alerting rules page (which only queries Prometheus). Verify your alert using the CLI:"
> ```bash
> oc get alertingrule -n user3-observability-demo
> oc describe alertingrule error-rate-alert -n user3-observability-demo
> ```

**Fix**: Correct the verification step in Exercise 5 to use the CLI check instead of the console Alerting page. Optionally add a note explaining the Prometheus-vs-Loki ruler distinction in the console.

---

## What worked well

- **LokiStack is fully operational**: All pods running, no configuration issues.
- **LogQL query examples**: Accurate, copy-paste-ready, and produce visible results.
- **AlertingRule YAML**: Correct syntax, rule fires as expected against incoming log traffic.
- **Log Browser (correct URL)**: Once navigated to the right URL, the log search works well and namespace filtering is clear.

---

---

## Module 3: Distributed Tracing and OpenTelemetry — Lab Evaluation

Evaluated by running through all exercises against cluster `apps.cluster-xwmfw.dynamic.redhatworkshops.io` as `user3` on April 8, 2026.

---

## Summary

All 9 exercises completed successfully. The distributed tracing stack (Tempo + OTel Operator) is fully operational. 4-hop traces (frontend → backend → notifier → database) are confirmed visible in the OCP console Traces UI. Three findings were identified — one structural inaccuracy in the lab guide about pod container counts (due to K8s Native Sidecar mode), one URL clarification, and one operational note about rollout ordering.

---

## Screenshots captured

| File | Exercise | Description |
|---|---|---|
| `module3-01-tempo-pods.png` | Ex1 | Tempo Stack all pods Running in `openshift-tempo-operator` namespace |
| `module3-03-traces-scatter.png` | Ex3/Ex7 | Traces scatter plot — Tempo instance `openshift-tempo-operator/tempo`, tenant `dev` |
| `module3-04-trace-waterfall.png` | Ex7 | 3-hop trace waterfall: frontend→backend→database (74ms, 6 spans) |
| `module3-05-span-attributes.png` | Ex7 | Database span attributes: baggage context, `db.operation`, `db.sql.table`, `db.system` |
| `module3-06-trace-4hop-notifier.png` | Ex9 | **4-hop trace**: frontend→backend→notifier (Python)→database (12 spans, 308ms) |
| `module3-07-notifier-span-attributes.png` | Ex9 | Notifier span attributes: `http.route=/notify`, `http.server_name=notifier:8083`, `http.status_code=200` |

---

## Exercises Completed

| Exercise | Description | Status |
|---|---|---|
| Ex1 | Tempo operator and TempoStack verified — all component pods Running | ✅ |
| Ex2 | Code review: `telemetry/telemetry.go` and `src/enable-otel.yaml` | ✅ |
| Ex3 | OTel operator and `central-collector` verified (2/2 replicas) | ✅ |
| Ex4 | Sidecar `OpenTelemetryCollector` CR created in `user3-observability-demo` | ✅ |
| Ex5 | `Instrumentation` CR `my-instrumentation` created with parentbased sampler | ✅ |
| Ex6 | OTel env vars and sidecar annotations applied to frontend/backend/database | ✅ |
| Ex7 | Traces confirmed in console — 3-hop waterfall screenshot taken | ✅ |
| Ex8 | Central collector config inspected (4 export destinations: Tempo, Thanos, Loki, Perses) | ✅ |
| Ex9 | Notifier Python auto-instrumentation enabled — 4-hop traces confirmed | ✅ |

---

## Findings

### M3-1: Native Sidecar mode changes pod container count — lab guide inaccurate (Bug — MEDIUM)

**Found in**: Exercise 6, verification step, and Exercise 9 verification step.

The lab guide instructs learners to verify that pods show two containers after sidecar injection:

> "Confirm the pod now has two containers (application + sidecar)"

And shows expected output:
```
NAME                     CONTAINERS
notifier-xxxxx           notifier, otc-container
```

**Actual behavior**: The OTel Operator is running in **K8s Native Sidecar mode** (K8s 1.29+ / OCP 4.21+). In native sidecar mode, the `otc-container` is injected as an **init container with `restartPolicy: Always`**, not as a regular container. As a result:

```bash
# This shows ONLY the application container (1 container):
oc get pods -o custom-columns='NAME:.metadata.name,CONTAINERS:.spec.containers[*].name'
# NAME                           CONTAINERS
# frontend-xxx                   frontend

# The sidecar is in initContainers:
oc get pods -o custom-columns='NAME:.metadata.name,INIT_CONTAINERS:.spec.initContainers[*].name'
# NAME                           INIT_CONTAINERS
# frontend-xxx                   otc-container
```

The sidecar IS running and functional — it is just visible in a different field. The lab guide's verification command using `.spec.containers[*].name` will confuse learners into thinking the sidecar injection failed.

**Before**:
```bash
oc get pods -o custom-columns='NAME:.metadata.name,CONTAINERS:.spec.containers[*].name'
```
Expected: `notifier, otc-container`

**After**:
```bash
oc get pods -o custom-columns='NAME:.metadata.name,CONTAINERS:.spec.containers[*].name,INIT_CONTAINERS:.spec.initContainers[*].name'
```
Expected: `CONTAINERS=notifier  INIT_CONTAINERS=otc-container`

Add a note to the lab guide explaining that OCP 4.21+ uses K8s Native Sidecar mode, which places the OTel sidecar in `spec.initContainers` with `restartPolicy: Always`. This is an improvement in stability over the previous regular-container sidecar approach.

**Fix**: Update all verification commands in Ex6 and Ex9 to include `spec.initContainers[*].name` in the custom-columns output. Add an informational callout box explaining native sidecar mode.

---

### M3-2: Traces page correct URL (Info — LOW)

**Found in**: Exercise 7, navigation step.

The Traces page is accessible at `Observe → Traces`, which correctly navigates to:

```
/observe/traces
```

However, if a user manually types `/monitoring/traces` (a common assumption from the `/monitoring/logs` URL in Module 2), they are **redirected to the Alerting page** rather than the Traces page. The guide correctly says "Navigate to Observe → Traces" but does not provide the path — add it for clarity.

**Fix**: Add the explicit URL path `/observe/traces` to the navigation instruction in Exercise 7.

---

### M3-3: Rollout restart required after sidecar CR creation (Operational Note — LOW)

**Found in**: Exercise 6.

When the `OpenTelemetryCollector` sidecar CR is created at the same time as OTEL env vars + annotations are applied (via `oc set env` + `oc patch`), the OTel Operator may not yet be ready to inject the sidecar before the deployment rollout completes. Pods roll out without the sidecar (because the OTC CR wasn't admitted yet by the admission webhook) and need to be restarted:

```bash
oc rollout restart deployment/frontend deployment/backend statefulset/database
```

**Fix**: Add an explicit `oc rollout restart` step after annotating deployments. Include a note explaining that the OTel admission webhook needs to observe the sidecar CR before pod scheduling.

---

### M3-4: Python auto-instrumentation — traces appear after traffic generation (Info — LOW)

**Found in**: Exercise 9.

After patching the notifier deployment with `instrumentation.opentelemetry.io/inject-python: "my-instrumentation"` and `sidecar.opentelemetry.io/inject: "sidecar"`, the Python OTel SDK auto-instrumentation works correctly via PYTHONPATH injection (`sitecustomize.py` mechanism). However, traces do not appear immediately — the BatchSpanProcessor buffers spans and the first export may take up to 10 seconds after traffic is generated.

**Verified working**: Sidecar metrics confirm 1,259 spans received via HTTP and 1,243 spans exported to the central collector. The 4-hop trace is visible in the console showing:
- `notifier: POST /notify` (206.8ms)
- `notifier: POST /notify http receive` (105μs)
- `notifier: POST` (httpx call to database, 19.12ms)
- `database: POST /events` (13.99ms)
- `notifier: POST /notify http send` (72μs/19μs)

**Fix**: Add a note in Exercise 9 to wait ~15 seconds after generating traffic before checking the Traces UI. Mention the batch export delay is determined by the `schedule_delay_millis` (default: 5000ms) on the `BatchSpanProcessor`.

---

## What worked well

- **Tempo Stack deployment**: Fully operational, all 8+ component pods Running, no configuration issues.
- **TraceQL queries**: Work out of the box; Tempo `dev` tenant correctly isolates traces.
- **3-hop trace propagation**: W3C TraceContext propagation across Go services (frontend→backend→database) works seamlessly.
- **Python auto-instrumentation**: The PYTHONPATH/sitecustomize.py mechanism correctly instruments a FastAPI/Uvicorn app with zero code changes. httpx client spans, ASGI server spans, and baggage propagation all confirmed.
- **Baggage propagation**: `baggage.client.platform=web` and `baggage.request.source=workshop-demo` correctly appear in database spans (set at frontend, propagated through all hops).
- **Native sidecar stability**: K8s native sidecar mode is more robust than the previous approach — the OTel collector starts before the application container and is visible in pod conditions.
- **OCP console Traces UI**: Clean, functional interface for Tempo. Scatter plot, waterfall view, and span attribute panel all work correctly.
- **Central collector config**: The pipeline correctly routes traces to both Tempo (gRPC) and spans appear in the 4-hop trace with correct k8s resource attributes injected by the k8sattributes processor.

---

## Verified resource checklist

| Resource | Namespace | Status |
|---|---|---|
| `TempoStack/tempo` | `openshift-tempo-operator` | ✅ Running (v2.10.0, all components) |
| `OpenTelemetryCollector/central-collector` | `observability-demo` | ✅ Running (2/2 replicas) |
| `OpenTelemetryCollector/sidecar` | `user3-observability-demo` | ✅ Created (mode: sidecar) |
| `Instrumentation/my-instrumentation` | `user3-observability-demo` | ✅ Created |
| `Deployment/frontend` + `backend` (with sidecar) | `user3-observability-demo` | ✅ Sidecar injected (init container) |
| `StatefulSet/database` (with sidecar) | `user3-observability-demo` | ✅ Sidecar injected (init container) |
| `Deployment/notifier` (Python auto-instrumented) | `user3-observability-demo` | ✅ Python SDK active, traces flowing |
| Traces visible in OCP console | `openshift-tempo-operator/tempo` tenant `dev` | ✅ 4-hop trace confirmed |

---

## What worked well

- **Step sequencing**: The exercises build on each other naturally — deploy → scrape → visualize → alert.
- **Debug sidecar pattern**: The UBI minimal debug sidecar for `curl` is elegant and realistic. Clear explanation of why it's used.
- **CMO vs COO distinction**: The comparison table between User Workload Monitoring and COO is well-written and accurately reflects the architectural difference.
- **Traffic generation loop**: The mixed 70/20/10 error ratio script produces realistic RED method data that shows up cleanly in the Perses panels.
- **Perses dashboard YAML**: The dashboard comes up working immediately in the console — all 3 panels render with live data.
- **PrometheusRule syntax**: Correct and complete, fires against the right namespace label.
- **Troubleshooting sections**: Well-organized. The RBAC issue is actually pre-documented — just needs to match current cluster state.

---

## Verified resource checklist

| Resource | Namespace | Status |
| --- | --- | --- |
| `Deployment/sample-app` (2 replicas, 2/2) | user3-observability-demo | ✅ Running |
| `Service/sample-app` | user3-observability-demo | ✅ Created |
| `ServiceMonitor/sample-app-monitor` (CMO) | user3-observability-demo | ✅ Created |
| `ServiceMonitor/sample-app-coo` (COO, `monitoring.rhobs/v1`) | user3-observability-demo | ✅ Created |
| `PersesDatasource/prometheus` (pre-provisioned) | user3-observability-demo | ✅ Available (v1alpha2) |
| `PersesDashboard/sample-app-dashboard` | user3-observability-demo | ✅ Visible in console |
| `PrometheusRule/sample-app-alerts-user3` | user3-observability-demo | ✅ Created |
| MonitoringStack `observability-stack` | observability-demo | ✅ Running (3x Prometheus, Thanos, Alertmanager) |
