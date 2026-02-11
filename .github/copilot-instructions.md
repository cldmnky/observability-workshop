# OpenShift Observability Workshop - AI Agent Instructions

## Project Overview

This repository serves dual purposes:
1. **Red Hat Showroom Workshop**: Antora-based hands-on learning content for OpenShift observability
2. **GitOps Infrastructure**: ArgoCD ApplicationSet + Helm charts deploying production observability stack

Deployed via showroom-deployer on RHDP (Red Hat Demo Platform) to OpenShift 4.21+ clusters.

## Architecture: Two Interconnected Systems

### Content System (Antora Documentation)
```
content/
  ├── antora.yml              # Site metadata, dynamic attributes
  ├── modules/ROOT/
  │   ├── nav.adoc            # Left navigation structure
  │   ├── pages/*.adoc        # Workshop modules (AsciiDoc format)
  │   └── assets/images/      # Screenshots, diagrams
  └── lib/                    # Antora extensions (dev-mode.js, attributes)
```

**Key Pattern**: Dynamic attribute injection from RHDP (e.g., `{openshift_cluster_console_url}`) replaced at runtime by showroom-deployer.

### Infrastructure System (GitOps Deployment)
```
bootstrap/
  ├── argocd/applicationset-observability.yaml  # Multi-app generator
  └── helm/
      ├── coo-monitoring-stack/     # Prometheus/Alertmanager (COO v1.3.1)
      ├── tempo-stack/              # Distributed tracing (Tempo v0.19.0-3)
      ├── opentelemetry-stack/      # Auto-instrumentation + collectors
      └── observability-servicemonitors/  # Metrics discovery
```

**Critical Pattern**: All Helm charts conditionally support ODF (OpenShift Data Foundation) or external S3:
- `storage.type: odf` → Auto-creates ObjectBucketClaim (NooBaa MCG)
- `storage.type: external` → Uses provided S3 credentials

## ArgoCD Sync Wave Orchestration

**Operators MUST install before CRs**. All charts follow this pattern:

```yaml
# Wave 0: Namespaces
# Wave 1: Operators (Subscription, OperatorGroup)
# Wave 2: RBAC, Secrets, ObjectBucketClaims
# Wave 3: CRs (MonitoringStack, TempoStack)
# Wave 4: UI Plugins (optional)
```

**Metadata on every resource**:
```yaml
metadata:
  annotations:
    argocd.argoproj.io/sync-wave: "1"  # Sequential deployment
    argocd.argoproj.io/sync-options: SkipDryRunOnMissingResource=true  # For CRDs
```

## Helm Chart Development Rules

### 1. Operator Install Mode (CRITICAL)
All operators in this stack use **AllNamespaces mode** and install to `openshift-operators`:

```yaml
# Correct pattern (COO, Tempo, OpenTelemetry operators)
operator:
  namespace: openshift-operators  # NOT custom namespace

# OperatorGroup must be conditional:
{{- if ne .Values.operator.namespace "openshift-operators" }}
# Only create OperatorGroup if NOT using openshift-operators
{{- end }}
```

### 2. ODF Storage Integration
When `storage.type: odf`, charts create ObjectBucketClaim (OBC):

```yaml
# ObjectBucketClaim generates two resources:
# - Secret: <bucketName> with AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY
# - ConfigMap: <bucketName> with BUCKET_NAME, BUCKET_HOST, BUCKET_PORT

# Tempo operator expects different secret format:
# Manual secret required: bucket, endpoint, access_key_id, access_key_secret
```

**Known Issue**: OBC secret fields don't match Tempo's expected schema. Workaround: Create manual secret from OBC values using HTTP endpoint (port 80, not 443/HTTPS) to avoid TLS verification issues with self-signed certs.

### 3. Schema Version Validation
Cluster Observability Operator v1.3.1 changed MonitoringStack schema:

```yaml
# WRONG (v1.2.x):
spec:
  prometheusConfig:
    resources:
      limits: {...}

# CORRECT (v1.3.1+):
spec:
  resources:  # Top-level, applies to all pods
    limits: {...}
```

Always check operator version when troubleshooting CRD validation errors.

## Claude Skills Workflow (Content Generation)

Located in `.claude/skills/`, these are AI-powered content creation tools:

### Primary Skills
- **create-lab**: Generate workshop modules from reference material (URLs, docs, text)
- **create-demo**: Build Know/Show presenter-led demos
- **verify-content**: Quality validation against Red Hat standards
- **blog-generate**: Convert completed modules to blog posts

### Usage Pattern (Sequential Q&A)
```bash
# In Cursor or Claude Code CLI:
/create-lab content/modules/ROOT/pages/

# AI asks ONE question at a time, builds content iteratively:
# 1. New lab or add to existing?
# 2. Story/business outcome?
# 3. Technical content sources?
# 4. Target audience? (Developers, Platform Engineers, Admins)
```

**Critical Rules** (from `.claude/skills/create-lab/SKILL.md`):
- **Sequential questioning**: Wait for user answer before next question
- **Reference enforcement**: All steps cite docs (no hallucination)
- **Version pinning**: Always use specific versions (e.g., `4.21`, `v0.19.0-3`)
- **Attribute placeholders**: Use `{openshift_cluster_console_url}` not hardcoded URLs
- **Navigation updates**: Always update `nav.adoc` after creating pages

## AsciiDoc Conventions (Workshop Content)

### File Naming Pattern
```
content/modules/ROOT/pages/
  ├── index.adoc                    # Landing page
  ├── 01-setup.adoc                 # Setup/prerequisites
  ├── 02-module-01-intro.adoc       # Module overview
  ├── 03-module-01-prometheus.adoc  # Detailed lab
  └── 04-module-01-wrapup.adoc      # Summary
```

**Numbering**: `XX-module-YY-topic.adoc` for page ordering.

### Required Attributes (antora.yml)
```yaml
asciidoc:
  attributes:
    lab_name: "openshift-observability-workshop"
    guid: my-guid  # Unique per user, injected by showroom-deployer
    openshift_cluster_console_url: https://console-openshift-console.apps.cluster.example.com
```

Reference in pages:
```asciidoc
Navigate to {openshift_cluster_console_url}[OpenShift Console^]
```

### Code Blocks Must Specify Language
```asciidoc
[source,bash]
----
oc get pods -n openshift-monitoring
----
```

Not `[source]` alone – breaks syntax highlighting.

## Local Development Commands

### Test Antora Site Build
```bash
# Using official Antora container
podman run --rm -ti -v ${PWD}:/antora registry.redhat.io/openshift4/ose-antora:latest \
  antora site.yml --to-dir www
```

### Validate Helm Charts
```bash
cd bootstrap/helm/tempo-stack
helm template . --values values.yaml | kubectl apply --dry-run=client -f -
```

### Test Claude Skills Locally
```bash
# In Cursor IDE: Open Command Palette → Type skill name
# Or in Claude Code CLI:
claude  # Then: /create-lab
```

## Debugging Deployment Issues

### ArgoCD Application Out of Sync
```bash
# Check specific app status
oc get application tempo-stack -n openshift-gitops -o yaml | yq '.status.conditions'

# Force sync
oc annotate application tempo-stack -n openshift-gitops argocd.argoproj.io/refresh=normal --overwrite
```

### Operator Not Installing (Common Issue)
```bash
# Check CSV (ClusterServiceVersion)
oc get csv -n openshift-operators | grep tempo

# Check InstallPlan
oc get installplan -n openshift-operators

# Operator stuck? Delete and let ArgoCD recreate:
oc delete subscription tempo-product -n openshift-operators
```

### TempoStack/MonitoringStack Not Reconciling
1. Get operator pod logs:
   ```bash
   oc logs -n openshift-operators deployment/tempo-operator-controller
   ```
2. Check CR status conditions:
   ```bash
   oc describe tempostack tempo -n openshift-tempo-operator | grep -A20 "Conditions:"
   ```
3. Common errors:
   - **Schema validation**: Wrong spec fields for operator version
   - **Secret missing fields**: ODF OBC secret vs. operator expected format
   - **TLS errors**: NooBaa port 443 requires TLS, use port 80 for HTTP

## Files to Never Modify Manually

- `content/modules/ROOT/nav.adoc` – Updated by Claude skills only
- Generated Antora assets in `www/` – Build output, not source
- `.cache/antora/` – Antora build cache

## Common Mistakes to Avoid

1. **Hardcoding cluster URLs** in AsciiDoc – Use `{openshift_cluster_console_url}` attribute
2. **Creating OperatorGroup in openshift-operators** – Global namespace already has one
3. **Missing sync waves** – CRs deploy before CRDs exist (add wave annotations)
4. **Using OBC secret directly** – Tempo requires reformatted secret fields
5. **Writing async questions** in Claude skills – Must be sequential Q&A
6. **Forgetting targetRevision** in ApplicationSet – Defaults to HEAD, specify `main`

## Testing Checklist Before PR

- [ ] All Helm charts render without errors: `helm template . --values values.yaml`
- [ ] Antora site builds: `podman run ... antora site.yml`
- [ ] Nav structure updated if pages added/removed
- [ ] AsciiDoc uses attribute placeholders, not hardcoded values
- [ ] ArgoCD sync waves set on all resources
- [ ] OpenShift 4.21+ compatibility verified (operator versions match)
