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
