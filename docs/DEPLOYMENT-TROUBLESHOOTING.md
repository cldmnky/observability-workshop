# Argo + Makefile Deployment Troubleshooting

This guide reflects the current workflow:

1. `make deploy` on a new cluster
2. ArgoCD deploys all apps (including showroom-site)
3. `make build` rebuilds site and user-info-api images
4. `make refresh` builds and rolls out latest images

## Required Inputs

Create the gitignored users file first:

```bash
cp .config/users.yaml.example .config/users.yaml
```

## Standard Flow

```bash
make deploy
make build
make refresh
```

## What `make deploy` Does

- Ensures namespace `showroom-workshop` exists
- Creates/updates secret `workshop-users-secret` from `.config/users.yaml`
- Applies `bootstrap/argocd/applicationset-observability.yaml`

## Common Issues

### `.config/users.yaml not found`

```bash
cp .config/users.yaml.example .config/users.yaml
```

Then edit `.config/users.yaml` with real values.

### BuildRun missing / builds fail

```bash
oc get build.shipwright.io -n showroom-workshop
oc get buildrun -n showroom-workshop --sort-by=.metadata.creationTimestamp
oc get pods -n showroom-workshop -l build.shipwright.io/name=showroom-site-build
oc get pods -n showroom-workshop -l build.shipwright.io/name=user-info-api-build
```

Use logs from Make targets:

```bash
make build-logs
make build-logs-api
```

### Showroom deployment not updating after builds

```bash
make refresh
oc rollout status deployment/showroom-site -n showroom-workshop
```

### User data stale after updating `.config/users.yaml`

```bash
make deploy
make refresh
```

### Argo app not syncing

```bash
oc get application -n openshift-gitops
oc get application showroom-site -n openshift-gitops -o yaml | yq '.status'
```

If needed, request a refresh:

```bash
oc annotate application showroom-site -n openshift-gitops argocd.argoproj.io/refresh=normal --overwrite
```

## Verification

```bash
make deploy-status
make url
oc get pods -n showroom-workshop -l app.kubernetes.io/name=showroom-site
```

Expected pod state for multi-user mode: `3/3` containers (`oauth-proxy`, `user-info-api`, `showroom-site`).
