# Showroom Site Helm Chart

Deploys the OpenShift Observability Workshop site using Builds for Red Hat OpenShift, with multi-user support and OAuth authentication.

## Features

- **Antora-based workshop content**: Static site built from AsciiDoc sources
- **Multi-user support**: OAuth proxy with user-specific data injection
- **Builds for OpenShift**: GitOps-driven container image builds using Shipwright
- **Workshop user RBAC**: Automated permissions for observability access
- **Image triggers**: Automatic deployment updates when new images are built

## Architecture

```
┌─────────────────────────────────────────────────┐
│ Route (TLS re-encrypt)                          │
└────────────────┬────────────────────────────────┘
                 │
┌────────────────▼────────────────────────────────┐
│ OAuth Proxy (Port 8443)                         │
│ - OpenShift OAuth authentication                │
│ - Sets user cookie                              │
└────────────────┬────────────────────────────────┘
                 │
    ┌────────────┴────────────┐
    │                         │
┌───▼──────────┐   ┌──────────▼─────────┐
│ Showroom Site│   │ User Info API      │
│ (nginx:8080) │   │ (Flask:8081)       │
│ - Antora HTML│   │ - Reads user secret│
│ - Injects JS │   │ - Returns user data│
└──────────────┘   └────────────────────┘
```

## Prerequisites

- OpenShift 4.21+
- Builds for Red Hat OpenShift operator installed
- OpenShift GitOps (ArgoCD) for automated deployment

## Configuration

### Core Settings

```yaml
namespace: showroom-workshop
createNamespace: false  # Let ArgoCD create it

build:
  source:
    git:
      url: https://github.com/cldmnky/observability-workshop.git
      revision: main
  output:
    image: image-registry.openshift-image-registry.svc:5000/showroom-workshop/showroom-site:latest
```

### Multi-User Configuration

```yaml
multiUser:
  enabled: true
  
  # User data secret (created by `make deploy` or manually)
  userDataSecret: workshop-users-secret
  
  userInfoAPI:
    enabled: true
    image: image-registry.openshift-image-registry.svc:5000/showroom-workshop/user-info-api:latest
  
  oauthProxy:
    enabled: true
```

### Workshop User RBAC

```yaml
workshopUsers:
  rbac:
    enabled: true  # Enable automated RBAC for workshop users
```

**What this creates**:

1. **ClusterRole** (`workshop-observability-access`): Read-only access to observability CRDs
2. **ClusterRoleBinding**: Grants ClusterRole to all authenticated users
3. **RoleBindings** in observability namespaces:
   - `observability-demo`: Edit access (create ServiceMonitors, deploy apps)
   - Operator namespaces: View-only access

## Deployment

### Via ArgoCD (Recommended)

Deploy the ApplicationSet which includes showroom-site:

```bash
oc apply -f bootstrap/argocd/applicationset-observability.yaml
```

### Via Helm (Local Testing)

```bash
# With user data secret
oc create secret generic workshop-users-secret \
  -n showroom-workshop \
  --from-file=users.yaml=.config/users.yaml

# Install chart
helm install showroom-site . \
  --namespace showroom-workshop \
  --create-namespace
```

### Via Makefile (Development)

```bash
# Create .config/users.yaml with workshop user data
cp .config/users.yaml.example .config/users.yaml
# Edit with real credentials

# Deploy everything (ArgoCD apps + user secret)
make deploy

# Trigger new builds
make build

# Build and refresh deployment
make refresh
```

## User Data Format

Create `.config/users.yaml` for workshop users:

```yaml
users:
  user1:
    console_url: "https://console-openshift-console.apps.cluster.example.com"
    login_command: "oc login -u user1 -p PASSWORD https://api.cluster.example.com:6443"
    openshift_cluster_ingress_domain: "apps.cluster.example.com"
    openshift_console_url: "https://console-openshift-console.apps.cluster.example.com"
    password: "PASSWORD"
    user: user1
  user2:
    # ... same structure
```

**Security Note**: `.config/users.yaml` is gitignored. Never commit credentials to git.

## RBAC Permissions

Workshop users get these permissions:

### ClusterRole (workshop-observability-access)

```yaml
rules:
# Console plugins
- apiGroups: [console.openshift.io]
  resources: [consoleplugins]
  verbs: [get, list]

# Monitoring resources
- apiGroups: [monitoring.coreos.com]
  resources: [prometheusrules, servicemonitors, podmonitors]
  verbs: [get, list, watch]

# Tempo traces
- apiGroups: [tempo.grafana.com]
  resources: [tempostacks]
  verbs: [get, list]

# OpenTelemetry
- apiGroups: [opentelemetry.io]
  resources: [instrumentations, opentelemetrycollectors]
  verbs: [get, list]
```

### Namespace RoleBindings

| Namespace | Role | Purpose |
|-----------|------|---------|
| `observability-demo` | `edit` | Create ServiceMonitors, deploy apps |
| `openshift-user-workload-monitoring` | `view` | Observe Prometheus pods |
| `openshift-tempo-operator` | `view` | Observe Tempo infrastructure |
| `openshift-logging` | `view` | Observe Loki infrastructure |
| `openshift-netobserv-operator` | `view` | Observe network monitoring |
| `openshift-cluster-observability-operator` | `view` | Observe UI plugins |

**No access to**:
- `openshift-monitoring` (platform monitoring)
- `openshift-operators` (cluster operators)
- Cluster-admin resources

## Builds Configuration

The chart creates two Build resources:

1. **showroom-site-build**: Builds the Antora site container
2. **user-info-api-build**: Builds the user info API container

Both use:
- **Strategy**: `buildah` ClusterBuildStrategy
- **Source**: Git repository (main branch)
- **Output**: Internal OpenShift registry

Builds are triggered via BuildRun resources, either:
- Automatically (initial build via Job)
- Manually (`make build`)
- On git webhook (future enhancement)

## Troubleshooting

### User context not loading

Check user-info-api logs:
```bash
oc logs -n showroom-workshop deployment/showroom-site -c user-info-api
```

Verify secret exists:
```bash
oc get secret workshop-users-secret -n showroom-workshop
```

### RBAC permissions not working

Check RoleBindings were created:
```bash
oc get rolebinding -n observability-demo | grep workshop
oc get clusterrolebinding | grep workshop
```

Verify user is authenticated:
```bash
oc whoami
```

### Build failing

Check BuildRun status:
```bash
oc get buildrun -n showroom-workshop
oc logs -n showroom-workshop -l build.shipwright.io/name=showroom-site-build
```

Check Build configuration:
```bash
oc describe build.shipwright.io showroom-site-build -n showroom-workshop
```

## Development

### Local Antora Build

Test Antora site generation locally:
```bash
podman run --rm -ti \
  -v ${PWD}:/antora \
  registry.redhat.io/openshift4/ose-antora:latest \
  antora site.yml --to-dir www
```

### Local Container Build

Build and test container locally:
```bash
make dev-build  # Build with podman
make dev-run    # Run on localhost:8080
```

## Values Reference

See [values.yaml](values.yaml) for complete configuration options.

Key values:

- `namespace`: Target namespace
- `build.source.git.url`: Git repository URL
- `build.source.git.revision`: Git branch/tag
- `multiUser.enabled`: Enable multi-user features
- `multiUser.userDataSecret`: Name of user data Secret
- `workshopUsers.rbac.enabled`: Enable workshop RBAC
- `route.host`: Custom route hostname (optional)

## Related Documentation

- [Parent Deployment Guide](../../../DEPLOYMENT.md)
- [ApplicationSet Configuration](../../argocd/applicationset-observability.yaml)
- [Makefile Commands](../../../Makefile)
