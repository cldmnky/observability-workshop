# Example Application

This directory contains a small four-service demo application used throughout the OpenShift Observability Workshop. The services are intentionally simple so that the focus stays on observability (traces, metrics, logs) rather than application logic.

## Architecture

```text
Browser
  └─► frontend  :8080   (Go)
          └─► backend   :8081   (Go)
                ├─► database  :8082   (Go)
                └─► notifier  :8083   (Python/FastAPI)
                        └─► database  :8082
```

All inter-service communication is plain HTTP. When OpenTelemetry auto-instrumentation is enabled (see [Enabling telemetry](#enabling-telemetry)), W3C `traceparent` headers propagate trace context across every hop.

---

## Services

### `frontend` — Go · port 8080

Serves the single-page web UI and acts as an API gateway to the backend.

| Route | Description |
| --- | --- |
| `GET /` | HTML shell (served from embedded `static/`) |
| `GET /ping` | Proxies to `backend /api/ok` |
| `GET /error` | Proxies to `backend /api/error` (triggers an error span) |
| `GET /events` | Proxies to `backend /api/events` |
| `GET /api/notes` | Proxies notes list from backend |
| `POST /api/notes` | Create a new note via backend |
| `GET /api/notes/:id` | Fetch a single note via backend |
| `PUT /api/notes/:id` | Update a note via backend |
| `DELETE /api/notes/:id` | Delete a note via backend |
| `GET /api/notes/export.md` | Export all notes as Markdown via backend |
| `GET /api/code` | Lists embedded source files (used by the Source Code tab) |
| `GET /api/code/*path` | Returns raw content of an embedded source file |
| `GET /healthz` | Health/readiness probe |

#### Frontend environment variables

| Variable | Default | Description |
| --- | --- | --- |
| `FRONTEND_ADDR` | `:8080` | Listen address |
| `BACKEND_URL` | `http://backend:8081` | Backend service URL |
| `SERVICE_NAME` | `frontend` | OTEL service name |
| `OTEL_ENABLED` | _(unset)_ | Set to `true` to activate telemetry |

---

### `backend` — Go · port 8081

Business logic layer. Handles the notes CRUD API and fan-out to the database and notifier.

| Route | Description |
| --- | --- |
| `GET /api/ok` | Returns 200 OK and records an event in the database |
| `GET /api/error` | Returns 500 and records an error event in the database |
| `GET /api/events` | Fetches the event log from the database |
| `GET /api/notes` | List all notes |
| `POST /api/notes` | Create a note (also calls notifier) |
| `GET /api/notes/:id` | Fetch a single note |
| `PUT /api/notes/:id` | Update a note (also calls notifier) |
| `DELETE /api/notes/:id` | Delete a note (also calls notifier) |
| `GET /api/notes/export.md` | Export all notes as Markdown |
| `GET /healthz` | Health/readiness probe |

#### Backend environment variables

| Variable | Default | Description |
| --- | --- | --- |
| `BACKEND_ADDR` | `:8081` | Listen address |
| `DATABASE_API_URL` | `http://database:8082` | Database service URL |
| `NOTIFIER_URL` | `http://notifier:8083` | Notifier service URL |
| `SERVICE_NAME` | `backend` | OTEL service name |
| `OTEL_ENABLED` | _(unset)_ | Set to `true` to activate telemetry |

---

### `database` — Go · port 8082

Embedded SQL database (ChaiSQL/Pebble) that persists the notes and event log. No external database dependency.

| Route | Description |
| --- | --- |
| `GET /notes` | List all notes |
| `POST /notes` | Create a note |
| `GET /notes/:id` | Fetch a single note |
| `PUT /notes/:id` | Update a note |
| `DELETE /notes/:id` | Delete a note |
| `GET /notes/export.md` | Export all notes as Markdown |
| `GET /events` | List all events |
| `POST /events` | Append an event |
| `GET /events/:id` | Fetch a single event |
| `GET /healthz` | Health/readiness probe |

#### Database environment variables

| Variable | Default | Description |
| --- | --- | --- |
| `DATABASE_ADDR` | `:8082` | Listen address |
| `DATABASE_FILE` | `/var/lib/chai/db` | Path to the on-disk database file |
| `SERVICE_NAME` | `database` | OTEL service name |
| `OTEL_ENABLED` | _(unset)_ | Set to `true` to activate telemetry |

---

### `notifier` — Python (FastAPI) · port 8083

Lightweight notification service that records note lifecycle events (created/updated/deleted) into the database. This service ships **without any OpenTelemetry SDK code** — it is the exercise target for the auto-instrumentation module of the workshop.

| Route | Description |
| --- | --- |
| `POST /notify` | Accepts a `{action, title, note_id}` payload and writes an event to the database |
| `GET /healthz` | Health/readiness probe |

#### Notifier environment variables

| Variable | Default | Description |
| --- | --- | --- |
| `DATABASE_API_URL` | `http://database:8082` | Database service URL |
| `SERVICE_NAME` | `notifier` | OTEL service name |

---

## Enabling telemetry

The Go services (frontend, backend, database) share a `telemetry` package that initialises the OTEL SDK when `OTEL_ENABLED=true`. Setting this variable alone is enough to activate traces, metrics, and log correlation; no code changes are required.

For the `notifier`, telemetry is added purely through the OpenTelemetry Operator's zero-code auto-instrumentation. Apply the `enable-otel.yaml` manifest (found in this directory) to patch all four deployments:

```bash
kubectl apply -f enable-otel.yaml
```

To remove instrumentation:

```bash
kubectl delete -f enable-otel.yaml
```

---

## Container images

Pre-built images are published to `quay.io/cldmnky/observability/` and tagged with `latest` and a short commit SHA:

| Service | Image |
| --- | --- |
| frontend | `quay.io/cldmnky/observability/frontend` |
| backend | `quay.io/cldmnky/observability/backend` |
| database | `quay.io/cldmnky/observability/database` |
| notifier | `quay.io/cldmnky/observability/notifier` |

Images are rebuilt automatically by the [build-push-images](.github/workflows/build-push-images.yml) GitHub Actions workflow on every push to `main` that touches `src/`.

---

## Building locally

All four services share a single Go module rooted at `src/`. The Go Containerfiles require the full `src/` directory as the build context:

```bash
# From the repository root
podman build -f src/backend/Containerfile   -t observability/backend   ./src
podman build -f src/frontend/Containerfile  -t observability/frontend  ./src
podman build -f src/database/Containerfile  -t observability/database  ./src

# Notifier is self-contained
podman build -f src/notifier/Containerfile  -t observability/notifier  ./src/notifier
```

## Deploying to OpenShift

The `deploy.yaml` manifest in this directory creates Shipwright `Build` and `BuildRun` resources that build all four images from source inside the cluster and deploy them as `Deployment`/`Service` pairs. Replace `__NAMESPACE__` with your target namespace:

```bash
sed 's/__NAMESPACE__/my-namespace/g' src/deploy.yaml | oc apply -f -
```
