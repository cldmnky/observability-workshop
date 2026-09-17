---
name: verify-logql-queries
description: Verify LogQL queries in workshop content against a live LokiStack as a workshop user. Use when testing, reviewing, or fixing LogQL in AsciiDoc labs, validating filter accuracy, debugging empty results, or checking AlertingRule expressions.
license: Apache-2.0
compatibility: opencode
metadata:
  domain: observability
  stack: openshift-logging-lokistack
---

# Verify LogQL Queries — LokiStack

Test every LogQL query from workshop content as a non-admin user (`user1`) with `logcli` before committing doc changes.

## Step 1 — Setup Auth (No Cluster Exec Needed)

The LokiStack gateway is public. Do NOT use bare gateway host — append the tenant path:

```bash
export NS="user1-observability-demo"  # replace user1 with target user
export GATEWAY="https://logging-loki-openshift-logging.apps.<cluster-domain>/api/logs/v1/application"
export TOKEN=$(oc whoami -t)
# Discover domain dynamically:
# oc get route logging-loki -n openshift-logging -o jsonpath='{.spec.host}'
```

Base invocation (note: NO `--org-id` — tenant comes from URL path):

```bash
logcli query --addr="$GATEWAY" --bearer-token="$TOKEN" --tls-skip-verify \
  -q --limit=5 --since=1h '{kubernetes_namespace_name="user1-observability-demo"} |json' -o raw
```

Bare `https://<gateway>` without `/api/logs/v1/application` returns `404 page not found`.

## Step 2 — Know the Real Log Format

Check `oc logs` first — docs go stale:

```bash
oc logs -n $NS deployment/frontend --tail=5
oc logs -n $NS deployment/backend --tail=5
```

Current app emits TWO styles:

- JSON access logs: `{"time":"...","msg":"access","service":"frontend","method":"GET","path":"/api/notes","status":200,"duration_ms":180,...}`
- Plaintext operational: `2026/09/17 13:54:00 INFO proxied request to backend method=POST path=/api/notes backend_status=201 ...`

In Loki, the collector wraps everything in a JSON envelope with a `message` field containing the original line (inner JSON is escaped). Correct precise pattern is **double-`|json`**:

```logql
{kubernetes_namespace_name="user1-observability-demo"}
|json | line_format "{{.message}}" |json | status=404
```

Both `| status=404` and `| status="404"` work (numeric normalized).

## Step 3 — Run the Standard Query Matrix

Replace `%OPENSHIFT_USERNAME%` with real user. Test log-line queries with `query`, metric queries with `query --step`:

```bash
# Q: precise 404s (must return ONLY status 404, e.g. {"path":"/not-found","status":404})
logcli query --addr="$GATEWAY" --bearer-token="$TOKEN" --tls-skip-verify -q --limit=3 --since=2h \
  "{kubernetes_namespace_name=\"$NS\"} |json | line_format \"{{.message}}\" |json | status=404" -o raw

# Q: ping 200s
logcli query --addr="$GATEWAY" --bearer-token="$TOKEN" --tls-skip-verify -q --limit=2 --since=2h \
  "{kubernetes_namespace_name=\"$NS\", kubernetes_pod_name=~\"frontend.*\"} |json | line_format \"{{.message}}\" |json | status=200 |= \"/ping\"" -o raw

# Q: rate with |~ (handles BOTH logfmt status= and JSON "status":)
logcli query --addr="$GATEWAY" --bearer-token="$TOKEN" --tls-skip-verify -q --since=1h --step=1m \
  'sum(rate({kubernetes_namespace_name="user1-observability-demo"} |~ `(?:status\s*=\s*404|\\?"status\\?"\s*:\s*404)` [5m]))'

# Q: unwrap latency (MUST be range query — instant at now is empty due to ingestion lag)
logcli query --addr="$GATEWAY" --bearer-token="$TOKEN" --tls-skip-verify -q --since=2h --step=1m \
  'avg by (kubernetes_pod_name) (avg_over_time({kubernetes_namespace_name="user1-observability-demo", kubernetes_pod_name=~"frontend.*"} |json | line_format "{{.message}}" |json | unwrap duration_ms [5m]))'

# Q: extraction
logcli query --addr="$GATEWAY" --bearer-token="$TOKEN" --tls-skip-verify -q --limit=2 --since=2h \
  "{kubernetes_namespace_name=\"$NS\"} |json | line_format \"{{.message}}\" |json | status=404 | line_format \"{{.method}} {{.path}} -> {{.status}}\""

# Alert expr (expect ~0.1-0.9/s true rate; bare |= gives ~26/s noise)
logcli query --addr="$GATEWAY" --bearer-token="$TOKEN" --tls-skip-verify -q --since=1h --step=1m \
  'sum(rate({kubernetes_namespace_name="user1-observability-demo"} |json | line_format "{{.message}}" |json | status=404 [5m]))'
```

## Step 4 — False-Positive Checklist (Common Doc Bugs)

| Bug pattern | Symptom | Fix |
|---|---|---|
| `\|= "404"` alone | Returns `200 OK` with port `:40456`, or `pod_id fd1404a3-...` | Use double-`\|json` + `\| status=404` |
| `\|= "200"` alone | Matches pod IDs, byte counts | Use `\| status=200` + `\|= "/ping"` |
| `\|json \| regexp "duration_ms=(...)"` | Empty unwrap `[]` — JSON has `"duration_ms":`, not `duration_ms=` | Use second `\|json` + `\| unwrap duration_ms` |
| `\|json \| regexp "status=(...)"` | Empty — same JSON mismatch | Use double-`\|json` |
| Old logfmt timestamp regexp | Returns `- -` | Use `\| line_format "{{.method}} {{.path}} -> {{.status}}"` |
| `logcli instant-query` for unwrap | `[]` even when data exists | Use `logcli query --since=2h --step=1m` (range) |
| Alert `\|= "404"` | `~26/s`, always firing | Use precise filter (`~0.1-0.9/s`) |

Verify no false positives by grepping results:

```bash
# Must show ONLY status 404, no 200 OK with :40456 ports
logcli query ... "{NS} |json | line_format \"{{.message}}\" |json | status=404" -o raw | grep -c '"status":404'
```

## Step 5 — Infra Gotchas

- **429 throttling / lag**: `1x.extra-small` Loki with many users lags 5–10 min for high-volume namespaces. Check: `oc logs -n openshift-logging -l app.kubernetes.io/component=collector --tail=50 | grep 429`. If Loki newest < `oc logs` newest, expand to `--since=2h`.
- **AlertingRule bash safety**: `cat <<EOF` (unquoted) executes backticks. Double-`|json` has no backticks (safe); `|~ \`...\`` has backticks (needs `<<'EOF'` or avoid in applied YAML).
- **Dry-run AlertingRule**: `cat <<EOF | oc apply --dry-run=client -f -`
- **404 paths** (k6 `ERROR_PATHS`): `/not-found`, `/api/does-not-exist`, `/broken`. `/error` forwards to backend `/api/error`.

## Pass Criteria

- [ ] Every doc query runs without parse error as `user1`
- [ ] 404 queries return ONLY `"status":404` lines (no `:40456`, no pod IDs)
- [ ] Unwrap returns numeric series as range query
- [ ] Alert expr returns true 404 rate (<2/s), not noise (>10/s)
- [ ] Sample `oc logs` output in doc matches real JSON + plaintext format
