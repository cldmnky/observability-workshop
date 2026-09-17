---
name: verify-traceql-queries
description: Verify TraceQL queries in workshop content against a live TempoStack as a workshop user. Use when testing, reviewing, or fixing TraceQL in AsciiDoc labs, validating span attributes, debugging empty results, or checking baggage/peer/db/note filters.
license: Apache-2.0
compatibility: opencode
metadata:
  domain: observability
  stack: openshift-tempo-distributed-tracing
---

# Verify TraceQL Queries — TempoStack

Test every TraceQL query from workshop content as a non-admin user (`user1`) with `podman run grafana/tempo-cli` before committing doc changes.

## Step 1 — Setup Auth (Gateway Route, No Port-Forward)

`user1` cannot `port-forward` in `openshift-tempo-operator`. Use the public gateway route with bearer token:

```bash
export GW=$(oc get route tempo-tempo-gateway -n openshift-tempo-operator -o jsonpath='{.spec.host}')
export TOKEN=$(oc whoami -t)
```

Tempo API path is `/api/traces/v1/<tenant>/tempo/api/...` (`/api/traces/v1/dev/search` alone serves Jaeger UI HTML):

```bash
# Smoke test via curl:
curl -sk -H "Authorization: Bearer $TOKEN" \
  "https://$GW/api/traces/v1/dev/tempo/api/search?q=%7Bresource.k8s.namespace.name%3D%22user1-observability-demo%22%7D" | head -c 300
```

Canonical `tempo-cli` invocation via podman:

```bash
podman run --rm docker.io/grafana/tempo-cli query api search "$GW" \
  '{resource.k8s.namespace.name="user1-observability-demo"}' 'now-1h' 'now' \
  --secure --path-prefix='/api/traces/v1/dev/tempo' --header="Authorization=Bearer $TOKEN" --limit=5
```

Tenant is `dev` (TempoStack `mode: openshift`, tenants `dev`/`prod`). Console uses Tempo instance `openshift-tempo-operator/tempo`, Tenant `dev`.

## Step 2 — Run the 8-Query Matrix

Replace `%OPENSHIFT_USERNAME%` with real user. All should return >0 traces:

```bash
run_q() { podman run --rm docker.io/grafana/tempo-cli query api search "$GW" "$1" 'now-1h' 'now' \
  --secure --path-prefix='/api/traces/v1/dev/tempo' --header="Authorization=Bearer $TOKEN" --limit=2 \
  2>&1 | python3 -c "import sys,json; print(len(json.load(sys.stdin).get('traces',[])))"; }
run_q '{resource.k8s.namespace.name="user1-observability-demo" && span.baggage.client.platform="web"}'
run_q '{resource.k8s.namespace.name="user1-observability-demo" && span.baggage.request.source="workshop-demo"}'
run_q '{resource.k8s.namespace.name="user1-observability-demo" && resource.service.name="database" && duration>50ms}'
run_q '{resource.k8s.namespace.name="user1-observability-demo" && span.db.sql.table="notes" && duration>30ms}'
run_q '{resource.k8s.namespace.name="user1-observability-demo" && span.db.operation="INSERT" && resource.service.name="database"}'
run_q '{resource.k8s.namespace.name="user1-observability-demo" && span.note.id > 0}'
run_q '{resource.k8s.namespace.name="user1-observability-demo" && span.peer.service="database" && span.db.sql.table="notes"}'
run_q '{resource.k8s.namespace.name="user1-observability-demo" && status=error}'
```

## Step 3 — Inspect Full Trace to Validate Attributes

`search` returns summaries only. Fetch full attribute dump for one trace:

```bash
TRACE_ID=<id-from-search>
curl -sk -H "Authorization: Bearer $TOKEN" \
  "https://$GW/api/traces/v1/dev/tempo/api/traces/$TRACE_ID" | python3 -c "
import sys,json
d=json.load(sys.stdin)
for b in d.get('batches',[]):
  rs={a['key']: list(a['value'].values())[0] for a in b.get('resource',{}).get('attributes',[])}
  print(f\"=== service={rs.get('service.name')} ===\")
  for ss in b.get('scopeSpans',[]):
    for s in ss.get('spans',[]):
      print(f\" span {s.get('name')} status={s.get('status',{}).get('code','UNSET')}\")
      for a in sorted(s.get('attributes',[]), key=lambda x: x['key']):
        print(f\"   {a['key']}={list(a['value'].values())[0]}\")
"
```

## Step 4 — Known Ground Truth (Don't "Fix" These)

Verified against live `user1-observability-demo` traces — doc must match this reality:

| Fact | Detail |
|---|---|
| HTTP semconv is NEW | `http.request.method`, `http.response.status_code`, `url.path`, `server.address`. Old `http.method` / `http.status_code` / `http.route` do NOT exist on spans. |
| Baggage recorded downstream only | Frontend *sets* `client.platform=web`, `request.source=workshop-demo` (see `src/frontend/main.go: baggageMiddleware`) but does NOT attach to own span. Backend/database record via `baggage.FromContext` + `SetAttributes`. Trace-level `span.baggage.*` queries still match (via downstream spans). |
| `note.id` is NUMERIC | `span.note.id != ""` returns **0 traces**. Use `span.note.id > 0` (or `!= nil`). |
| `/ping` HAS database INSERTs | `/ping` → backend `/api/ok` → database `POST /events` (`db.operation=INSERT`, `table=events`). So `db.operation=INSERT` matching `GET /ping` traces is CORRECT. |
| `status=error` works | Finds `STATUS_CODE_ERROR` spans (e.g. `database POST /events` error). |
| Notifier 0 traces pre-Ex9 | `oc get pods -n $NS -l app=notifier -o custom-columns=INIT:.spec.initContainers[*].name` shows `<none>` until Exercise 9 annotation. Expected. |
| Waterfall names | Server spans `POST /api/notes`, client spans `HTTP POST`; console prefixes with service (`frontend: POST ...`). Backend has TWO `HTTP POST` children (database + notifier after Ex9). |

## Step 5 — Common Doc Bugs

| Bug | Symptom | Fix |
|---|---|---|
| `span.note.id != ""` | 0 traces | `span.note.id > 0` + note explaining numeric type |
| Frontend table lists `baggage.*` on frontend span | Learner can't find them on frontend span | Move baggage to backend/database tables; note frontend sets, downstream records |
| Old `http.method` names | Learner searches attributes panel, not found | Use new names, mention old as legacy |
| Claiming `/ping`+INSERT is wrong | False bug report | Document events-table INSERT on `/ping` path |
| Forgetting namespace scope | User sees other users' traces (shared backend) | Every query MUST include `{ resource.k8s.namespace.name = "<ns>" }` |

## Pass Criteria

- [ ] All doc TraceQL return >0 traces as `user1` via podman tempo-cli
- [ ] `note.id` uses numeric comparison
- [ ] Attribute tables use `http.request.method` / `http.response.status_code` / `url.path`
- [ ] Baggage documented as set-on-frontend, recorded-downstream
- [ ] Every query scopes `resource.k8s.namespace.name`
