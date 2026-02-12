# Multi-User Workshop Configuration

This directory contains the implementation for multi-user workshop support with dynamic user-specific content injection.

## Architecture

```
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
│  │ ConfigMap: workshop-users                              │  │
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
- Returns user-specific data from ConfigMap
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
- `user-data-configmap.yaml` - User data from values.yaml
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

users:
  user1:
    console_url: "https://console-openshift-console.apps.cluster.example.com"
    password: "user1password"
    login_command: "oc login -u user1 -p user1password https://api.cluster.example.com:6443"
    openshift_cluster_ingress_domain: "apps.cluster.example.com"
  user2:
    console_url: "https://console-openshift-console.apps.cluster.example.com"
    password: "user2password"
    login_command: "oc login -u user2 -p user2password https://api.cluster.example.com:6443"
    openshift_cluster_ingress_domain: "apps.cluster.example.com"
```

## Deployment

### With Multi-User Enabled

```bash
# 1. Build user-info-api (first time only)
oc create -f - <<EOF
apiVersion: shipwright.io/v1beta1
kind: BuildRun
metadata:
  generateName: user-info-api-build-
  namespace: showroom-workshop
spec:
  build:
    name: user-info-api-build
EOF

# 2. Wait for user-info-api image build
oc logs -f -n showroom-workshop $(oc get pods -n showroom-workshop -l build.shipwright.io/name=user-info-api-build --sort-by=.metadata.creationTimestamp -o jsonpath='{.items[-1].metadata.name}')

# 3. Deploy/upgrade chart
helm upgrade --install showroom-site bootstrap/helm/showroom-site \
  --namespace showroom-workshop \
  --create-namespace \
  --values bootstrap/helm/showroom-site/values.yaml

# 4. Check deployment
oc get pods -n showroom-workshop
# Should see 3 containers per pod: oauth-proxy, user-info-api, showroom-site
```

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
export USER_DATA_FILE=../bootstrap/helm/showroom-site/values.yaml
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

**Check ConfigMap:**
```bash
oc get configmap workshop-users -n showroom-workshop -o yaml
```

**Trigger ConfigMap reload:**
```bash
# Edit values.yaml and upgrade chart
helm upgrade showroom-site bootstrap/helm/showroom-site \
  --namespace showroom-workshop \
  --values bootstrap/helm/showroom-site/values.yaml
```

## Security Considerations

- Passwords are stored in ConfigMap (not encrypted at rest)
- For production, consider:
  - External secret management (Vault, External Secrets Operator)
  - Set `HIDE_PASSWORDS=true` in user-info-api
  - Use temporary passwords
  - Rotate credentials regularly

## Integration with RHDP/AgnosticV

When deploying via showroom-deployer on RHDP, the user data can be injected from AgnosticV variables:

```yaml
# In AgnosticV output_vars
showroom_users: "{{ users | to_json }}"

# Chart values can reference this
users:
  {{ showroom_users }}
```
