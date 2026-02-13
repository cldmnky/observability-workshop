# Multi-User Workshop Configuration

This directory contains the implementation for multi-user workshop support with dynamic user-specific content injection.

## Architecture

```text
┌─────────────────────────────────────────────────────────────┐
│ User Browser                                                 │
│  ↓                                                           │
│ 1. Accesses Route → OpenShift OAuth Login                   │
│ 2. OAuth Redirects → User Authenticates                     │
│ 3. OAuth Proxy validates → Injects X-Forwarded-User header  │
│ 4. Static site loads → JavaScript fetches /api/user-info    │
│ 5. User Info API reads header → Returns user-specific data  │
│ 6. JavaScript replaces {placeholders} with real values      │
└─────────────────────────────────────────────────────────────┘

Pod Components:
┌──────────────────────────────────────────────────────────────┐
│ showroom-site Pod                                             │
│                                                               │
│  ┌────────────────┐  ┌─────────────────┐  ┌──────────────┐  │
│  │ OAuth Proxy    │  │ User Info API   │  │ nginx        │  │
│  │ (port 8443)    │→ │ (port 8081)     │  │ (port 8080)  │  │
│  │                │  │                 │  │              │  │
│  │ - Auth users   │  │ - Read headers  │  │ - Serve site │  │
│  │ - Inject header│  │ - Return JSON   │  │ - Static HTML│  │
│  └────────────────┘  └─────────────────┘  └──────────────┘  │
│           ↓                   ↓                               │
│  ┌────────────────────────────────────────────────────────┐  │
│  │ Secret: workshop-users-secret                          │  │
│  │ - user1: {password, console_url, login_command}        │  │
│  │ - user2: {password, console_url, login_command}        │  │
│  └────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────┘
```

## Components

### 1. Client-Side JavaScript (`user-context.js`)

- Loaded on every page via supplemental-ui
- Fetches `/api/user-info` endpoint
- Replaces placeholders: `{user}`, `{password}`, `{console_url}`, `{login_command}`
- Shows user badge in header

### 2. User Info API Server (`user-info-api/`)

- Python Flask application
- Reads `X-Forwarded-User` header from OAuth proxy
- Returns user-specific data from Secret-mounted `users.yaml`
- Endpoints:
  - `GET /api/user-info` - Current user's info
  - `GET /api/users` - List all users
  - `GET /healthz` - Health check

### 3. OAuth Proxy Sidecar

- OpenShift origin-oauth-proxy
- Enforces OpenShift authentication
- Injects `X-Forwarded-User` header
- Terminates TLS

### 4. Helm Chart Templates

- User data Secret is managed outside the chart via `make deploy`
- `user-info-api-build.yaml` - Builds user-info-api container
- `oauth-serviceaccount.yaml` - ServiceAccount for OAuth
- `oauth-tls-secret.yaml` - TLS secret for OAuth proxy
- Updated `deployment.yaml` - Adds sidecars
- Updated `service.yaml` - Exposes API and OAuth ports
- Updated `route.yaml` - Routes to OAuth proxy

## Configuration

Edit `bootstrap/helm/showroom-site/values.yaml`:

```yaml
multiUser:
  enabled: true  # Enable multi-user support
  
  userInfoAPI:
    enabled: true
    image: image-registry.openshift-image-registry.svc:5000/showroom-workshop/user-info-api:latest
  
  oauthProxy:
    enabled: true
```

Create user data in `.config/users.yaml` (gitignored):

```bash
cp .config/users.yaml.example .config/users.yaml
```

Then update users in `.config/users.yaml`.

## Deployment

### With Multi-User Enabled

```bash
# 1. Bootstrap ArgoCD and users data
make deploy

# 2. Build both images in cluster
make build

# 3. Refresh deployment with latest images
make refresh

# 4. Check deployment
oc get pods -n showroom-workshop
# Should see 3 containers per pod: oauth-proxy, user-info-api, showroom-site
```

`make deploy` also creates:

- OpenShift group `workshop-users` containing all users in `.config/users.yaml`
- Namespaced `view` RoleBindings for `workshop-users` in operator namespaces
- Per-user exercise namespaces (`<user>-observability-demo`, `<user>-tracing-demo`) with `edit` for each matching user

### Disable Multi-User (Simple Deployment)

```yaml
multiUser:
  enabled: false
```

This falls back to single-user mode with edge TLS termination.

## Supported Placeholders

In Antora content (`.adoc` files), use these placeholders:

- `{user}` - Current username
- `{password}` - User's password
- `{console_url}` or `{openshift_console_url}` - OpenShift console URL
- `{login_command}` - Complete oc login command
- `{openshift_cluster_ingress_domain}` - Cluster ingress domain
- `{api_url}` - OpenShift API URL

Exercise namespaces are automatically personalized in workshop pages: `observability-demo` and `tracing-demo` render as `<user>-observability-demo` and `<user>-tracing-demo` for the logged-in user.

Example:

```asciidoc
Login to OpenShift:

* Console: {openshift_console_url}
* Username: {user}
* Password: {password}

Or via CLI: `{login_command}`
```

## Testing Locally

```bash
# Run user-info-api locally
cd user-info-api
export USER_DATA_FILE=../.config/users.yaml
pip install -r requirements.txt
python app.py &

# Test API
curl http://localhost:8081/api/user-info?user=user1
curl http://localhost:8081/api/users

# Test with OAuth header simulation
curl -H "X-Forwarded-User: user2" http://localhost:8081/api/user-info
```

## Troubleshooting

### Users see placeholders instead of real values

**Check:**

1. Browser console for JavaScript errors
2. `/api/user-info` endpoint is accessible
3. OAuth proxy is injecting `X-Forwarded-User` header

```bash
# Check user-info-api logs
oc logs -n showroom-workshop -l app.kubernetes.io/component=web -c user-info-api

# Test API endpoint
oc exec -n showroom-workshop deployment/showroom-site -c showroom-site -- \
  curl -H "X-Forwarded-User: user1" http://localhost:8081/api/user-info
```

### OAuth authentication not working

**Check:**

1. ServiceAccount has OAuth annotation
2. Route uses reencrypt termination
3. TLS secret exists

```bash
# Check OAuth ServiceAccount
oc describe sa showroom-site-oauth -n showroom-workshop

# Check Route
oc describe route showroom-site -n showroom-workshop

# Check OAuth proxy logs
oc logs -n showroom-workshop -l app.kubernetes.io/name=showroom-site -c oauth-proxy
```

### User data not found

**Check Secret:**

```bash
oc get secret workshop-users-secret -n showroom-workshop -o yaml
```

**Update user data Secret:**

```bash
# Edit .config/users.yaml, then redeploy and refresh
make deploy
make refresh
```

## Security Considerations

- Passwords are stored in Secret (base64-encoded, encryption at rest depends on cluster config)
- For production, consider:
  - External secret management (Vault, External Secrets Operator)
  - Set `HIDE_PASSWORDS=true` in user-info-api
  - Use temporary passwords
  - Rotate credentials regularly

## Integration with RHDP/AgnosticV

When deploying via showroom-deployer on RHDP, generate `.config/users.yaml` from AgnosticV variables before running `make deploy`.

```yaml
# In AgnosticV output_vars
showroom_users: "{{ users | to_json }}"
```
