# 20-Minute Demo Flow: OpenShift Observability

## Setup Assumptions

- Sample applications are deployed and healthy.
- k6 load generator is running continuously.
- Tracing is enabled and OTel sidecars are injected.
- You are logged in to the OpenShift Console.

## Demo Timeline

| Segment | Focus | Time |
| --- | --- | --- |
| Act 1 | Detect with Perses dashboards | ~6 min |
| Act 2 | Locate bottleneck with traces | ~8 min |
| Act 3 | Investigate logs with Loki | ~5 min |
| Wrap-up | Final narrative and value | ~1 min |

---

## Act 1: Detect (Perses Dashboards) - ~6 min

### Act 1 Story

"I have four services under load. Let me check application health at a glance."

### Act 1 Flow

1. Open **OpenShift Console -> Observe -> Dashboards (Perses)**.
2. Select the **Sample Application Metrics** dashboard.
3. Highlight this point: native OpenShift UI, no separate Grafana endpoint.
4. Show **Request Rate** panel and call out real-time traffic by HTTP status code.
5. Show **Total Requests** panel and point to 200/404/500 mix from k6 load.

### Talk Track (GitOps)

"This dashboard is defined as a `PersesDashboard` CRD, versioned in Git, applied by Argo CD, and rendered by Cluster Observability Operator."

### Optional (1 min)

- Open standalone Perses UI briefly.
- Show visual editing.
- Explain: build interactively, export YAML, apply via GitOps.

---

## Act 2: Locate (OTel + Traces) - ~8 min

### Act 2 Story

"The dashboard shows elevated latency. Now I need to find where the delay lives in the call chain."

### Act 2 Flow

1. Explain two-tier OTel architecture:
     - App pod -> sidecar collector (`otc-container`) on localhost.
     - Sidecar -> central collector -> Tempo (traces), Prometheus (span metrics), Loki (logs).
2. Navigate to **Observe -> Traces**.
3. Show trace list with full path: frontend -> backend -> database -> notifier.
4. Open one trace in waterfall view.
5. Point out span duration, service name, and HTTP attributes.

### Talk Track (Bottleneck)

"This request took about 950 ms total, and backend -> database consumed about 880 ms. That is the bottleneck."

### TraceQL Queries (2 min)

```traceql
{ duration > 500ms }
```

```traceql
{ status = error }
```

### Span Attributes to Highlight

- `http.method`
- `http.route`
- `http.status_code`
- `k8s.pod.name`
- `k8s.namespace.name`

### Talk Track (Enrichment)

"The sidecar collector adds Kubernetes context to every span using the `k8sattributes` processor, without developer changes in each service."

---

## Act 3: Investigate (Loki Log Queries) - ~5 min

### Act 3 Story

"I found the slow span. Next I want the exact log lines from that service and timeframe."

### Act 3 Flow

1. Open **Observe -> Logs** and click **Show query**.
2. Start broad for all namespace logs:

```logql
{kubernetes_namespace_name="<namespace>"} | json
```

1. Narrow to backend errors:

```logql
{kubernetes_namespace_name="<namespace>", kubernetes_pod_name=~"backend.*"}
| json
| regexp `status=(?P<status>\d{3})`
| status="500"
```

### Act 3 Talk Track

"Use label selectors first, then parse, then filter. It is a very fast way to isolate failure logs."

### Optional (1 min): Log-Derived Rate

```logql
sum(rate(
    {kubernetes_namespace_name="<namespace>"}
    | json
    | regexp `status=(?P<status>\d{3})`
    | status="404" [5m]
))
```

### Talk Track

"You can compute error rates directly from logs, without building a separate custom metrics pipeline first."

---

## Wrap-Up - ~1 min

"Detect with dashboards, locate with traces, and investigate with logs. Three signals, one OpenShift experience, managed through OpenTelemetry and GitOps."

---

## Presenter Notes

- Keep the console full screen.
- Pre-open tabs for **Dashboards**, **Traces**, and **Logs**.
- Keep k6 load running throughout for live, changing data.
- Spend extra time on the trace waterfall because it is the strongest visual moment.
