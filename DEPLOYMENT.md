# Deployment Guide: OpenShift Observability Workshop

This guide covers deploying the OpenShift Observability Workshop on Red Hat Demo Platform (RHDP) using showroom-deployer.

## Prerequisites

- Access to Red Hat Demo Platform (RHDP)
- AgnosticV catalog item configured for this workshop
- OpenShift 4.21+ cluster provisioned via RHDP
- OpenShift GitOps (ArgoCD) installed on target cluster

## Deployment Architecture

```
RHDP Catalog Item
    |
    ├─→ Provisions OpenShift Cluster (via AgnosticV)
    ├─→ Deploys Showroom Interface (via showroom-deployer)
    └─→ Bootstraps Observability Stack (via ArgoCD)
```

## Step 1: Deploy via Showroom Deployer

The workshop is deployed using [showroom-deployer](https://github.com/rhpds/showroom-deployer), which handles building and serving the Antora-based content.

### AgnosticV Configuration

Your AgnosticV catalog item should include:

```yaml
# In your catalog's common.yaml or workload configuration
showroom_git_repo: https://github.com/rhpds/showroom_template_nookbag.git
showroom_git_ref: main
showroom_deployer_version: latest
```

### What Showroom-Deployer Does

1. Clones this repository
2. Builds the Antora site from `content/` directory
3. Deploys a pod serving the workshop content
4. Creates a route for learner access
5. Injects user-specific variables (URLs, credentials)

### User-Specific Variables

The workshop uses these dynamic attributes (populated by showroom-deployer):

- `{openshift_cluster_console_url}` - OpenShift console URL
- `{openshift_cluster_admin_username}` - Admin username  
- `{openshift_cluster_admin_password}` - Admin password
- `{openshift_api_url}` - OpenShift API endpoint

These are replaced at runtime with actual values from the provisioned cluster.

## Step 2: Bootstrap Observability Stack

After the workshop interface is deployed, bootstrap the observability infrastructure using ArgoCD.

### Option A: Automatic Bootstrap (Recommended)

Include the bootstrap in your AgnosticV workload:

```yaml
# In your AgnosticV workload post-deployment tasks
- name: Deploy observability stack
  kubernetes.core.k8s:
    state: present
    src: "{{ playbook_dir }}/files/applicationset-observability.yaml"
    namespace: openshift-gitops
```

Copy `bootstrap/argocd/applicationset-observability.yaml` to your workload's files directory.

### Option B: Manual Bootstrap

If learners will deploy the stack as part of the workshop:

1. Include instructions in module prerequisites
2. Provide the ApplicationSet manifest
3. Guide learners through deployment verification

**Command**:
```bash
oc apply -f bootstrap/argocd/applicationset-observability.yaml
```

### Verify Bootstrap Deployment

Check that all observability components are deployed:

```bash
# Check ArgoCD applications
oc get applications -n openshift-gitops

# Verify components
oc get pods -n openshift-user-workload-monitoring
oc get pods -n openshift-logging
oc get pods -n openshift-tempo-operator
oc get pods -n openshift-opentelemetry-operator
```

All applications should show `Healthy` and `Synced` status.

## Step 3: Verify Workshop Access

### Showroom URL

The showroom-deployer creates a route. Get the URL:

```bash
oc get route -n <showroom-namespace> -l app=showroom
```

### Test Workshop Content

1. Open the Showroom URL in a browser
2. Verify the home page loads
3. Check navigation shows all 4 modules
4. Confirm user variables are populated (not showing `{variable_name}`)

### Test Observability Stack

Follow Module 1, Exercise 1 to verify monitoring:

```bash
oc get pods -n openshift-user-workload-monitoring
```

Should show Prometheus, Thanos, and Alertmanager pods running.

## Troubleshooting

### Showroom Content Not Loading

**Issue**: 404 or page not found

**Solution**:
- Check showroom pod logs: `oc logs -n <namespace> -l app=showroom`
- Verify Antora build succeeded (no build errors in logs)
- Check route exists: `oc get route -n <namespace>`

### Variables Not Replaced

**Issue**: Content shows `{openshift_console_url}` instead of actual URL

**Solution**:
- Verify showroom-deployer injected variables
- Check user-info ConfigMap exists: `oc get configmap -n <namespace>`
- Ensure showroom version is up-to-date

### ArgoCD Applications Not Syncing

**Issue**: Applications stuck in "Progressing" or "Degraded"

**Solution**:
- Check application details: `oc describe application <app-name> -n openshift-gitops`
- View ArgoCD UI for detailed sync status
- Check Helm chart values are correct
- Verify storage class exists for PVCs

### Collector Pod Not Starting

**Issue**: OpenTelemetry or other pods crash-looping

**Solution**:
- Check pod events: `oc describe pod <pod-name> -n <namespace>`
- View pod logs: `oc logs <pod-name> -n <namespace>`
- Verify resource requests/limits are appropriate
- Check network policies aren't blocking traffic

## Customization

### Adjust Storage Sizes

Edit Helm values in `bootstrap/helm/<component>/values.yaml`:

```yaml
# Example: Increase Prometheus storage
prometheusUserWorkload:
  volumeClaimTemplate:
    spec:
      resources:
        requests:
          storage: 50Gi  # Default: 10Gi
```

### Change ArgoCD Sync Policy

Edit `bootstrap/argocd/applicationset-observability.yaml`:

```yaml
syncPolicy:
  automated:
    prune: false     # Disable auto-pruning
    selfHeal: false  # Disable auto-healing
```

### Add Custom Workloads

To include additional demo applications:

1. Create Helm chart in `bootstrap/helm/my-app/`
2. Add entry to ApplicationSet generators
3. Commit and push changes
4. ArgoCD will sync automatically

## Production Considerations

### Resource Requirements

Minimum cluster resources for full observability stack:

- **CPU**: 8 cores available
- **Memory**: 16 GB available
- **Storage**: 50 GB PV storage

### Multi-User Workshops

For workshops with multiple concurrent users, this repository provides built-in RBAC and multi-user support.

#### User Authentication & Data

The showroom-site deployment includes:

1. **OAuth Proxy**: Authenticates users via OpenShift OAuth
2. **User Info API**: Sidecar container serving user-specific data
3. **Dynamic Context Injection**: JavaScript replaces placeholders with user-specific values

**Setup**:
```bash
# Create users.yaml with workshop user data
cp .config/users.yaml.example .config/users.yaml
# Edit .config/users.yaml with real credentials

# Deploy with user data
make deploy
```

This creates a Secret (`workshop-users-secret`) with user data that the API reads at runtime.

#### Workshop User RBAC

The showroom-site Helm chart automatically configures RBAC permissions for authenticated workshop users:

**ClusterRole Permissions** (all authenticated users):
- Read access to observability CRDs (ServiceMonitors, TempoStacks, Instrumentations)
- Access to console plugins (monitoring, tracing, logging UIs)

**Namespace-Specific Permissions**:
- **observability-demo**: Full edit access (create/modify ServiceMonitors, deploy apps)
- **openshift-user-workload-monitoring**: View only (observe Prometheus pods)
- **openshift-tempo-operator**: View only (observe Tempo pods)
- **openshift-logging**: View only (observe Loki pods)
- **openshift-netobserv-operator**: View only (observe network monitoring)
- **openshift-cluster-observability-operator**: View only (observe UI plugins)

**Configure RBAC**:
```yaml
# In bootstrap/helm/showroom-site/values.yaml
workshopUsers:
  rbac:
    enabled: true  # Enable workshop RBAC (default: true)
```

**Security Model**:
- Users have full control in `observability-demo` workspace
- Users can observe (not modify) operator infrastructure
- Users cannot access platform monitoring (openshift-monitoring)
- Users cannot access cluster-admin resources

#### Scaling Considerations

For workshops with many concurrent users:

- Use separate namespaces per user
- Scale collectors based on expected load
- Implement namespace-based isolation
- Consider log retention policies

### Long-Running Workshops

For multi-day workshops:

- Adjust retention periods (Prometheus: 7d, Loki: 7d, Tempo: 7d)
- Monitor storage consumption
- Implement compaction schedules
- Plan for backup/restore if needed

## Support

**Documentation**:
- Main README: [../README.adoc](../README.adoc)
- Bootstrap Guide: [bootstrap/README.md](bootstrap/README.md)
- Showroom Deployer: https://github.com/rhpds/showroom-deployer

**Issues**:
- Workshop content: Submit issue to this repository
- Deployment issues: Contact RHDP support
- Showroom-deployer: Submit issue to showroom-deployer repository

## Reference Links

- **Showroom Deployer**: https://github.com/rhpds/showroom-deployer
- **AgnosticD**: https://github.com/redhat-cop/agnosticd
- **RHDP Documentation**: https://demo.redhat.com (internal)
- **OpenShift GitOps**: https://docs.openshift.com/gitops/latest/
