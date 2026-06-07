# OpenShift Observability Workshop - Project Rules

This file provides project-wide rules for AI assistants working in this repository. It is the opencode-native equivalent of the legacy `.github/copilot-instructions.md`.

## Project Overview

This repository serves dual purposes:

1. **Red Hat Showroom Workshop**: Antora-based hands-on learning content for OpenShift observability.
2. **GitOps Infrastructure**: ArgoCD ApplicationSet + Helm charts deploying a production observability stack.

Deployed via `showroom-deployer` on RHDP (Red Hat Demo Platform) to OpenShift 4.21+ clusters.

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

**Key Pattern**: Dynamic attribute injection from RHDP (e.g., `{openshift_cluster_console_url}`) replaced at runtime by `showroom-deployer`.

The actual Antora build lives under `showroom/` (see `showroom/site.yml` for the playbook and `showroom/content/` for the content tree). The `content/` at the repo root is the legacy/declarative view; the canonical site generator is `showroom/`.

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

- `storage.type: odf` → Auto-creates `ObjectBucketClaim` (NooBaa MCG).
- `storage.type: external` → Uses provided S3 credentials.

## ArgoCD Sync Wave Orchestration

**Operators MUST install before CRs.** All charts follow this pattern:

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

When `storage.type: odf`, charts create `ObjectBucketClaim` (OBC):

- OBC generates a Secret with `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`.
- OBC generates a ConfigMap with `BUCKET_NAME`, `BUCKET_HOST`, `BUCKET_PORT`.
- Tempo operator expects different fields: `bucket`, `endpoint`, `access_key_id`, `access_key_secret`.

**Known Issue**: OBC secret fields don't match Tempo's expected schema. Workaround: create a manual secret from OBC values using HTTP endpoint (port 80, not 443/HTTPS) to avoid TLS verification issues with self-signed certs. See the `argocd-helm-ops` skill for the exact commands.

### 3. Schema Version Validation

Cluster Observability Operator v1.3.1 changed `MonitoringStack` schema:

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

## AsciiDoc Conventions (Workshop Content)

### File Naming Pattern

```
showroom/content/modules/ROOT/pages/
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

Not `[source]` alone — breaks syntax highlighting.

## Specialized Agents and Skills

This repo defines a set of custom opencode agents and skills under `.opencode/`:

- **Agents** (`.opencode/agents/*.md`) — invoke with `@agent-name`:
  - `researcher` — research, validation, and synthesis for content.
  - `gitops-expert` — ArgoCD/Helm/ODF deployment and troubleshooting.
  - `technical-writer` — produces Red Hat Know/Show workshop and demo content.
  - `workshop-reviewer` — comprehensive quality review of workshop content.
  - `antora-site` — Antora build, supplemental UI, and `%TOKEN%` substitution.
  - `custom-agent-foundry` — designs new opencode agents.
- **Skills** (`.opencode/skills/*/SKILL.md`) — load on demand:
  - `argocd-helm-ops` — Helm/ArgoCD operational workflow for this stack.
  - `verify-*` / `enhanced-verification-*` — content quality rubrics (workshop + demo).
  - `redhat-style-guide-validation` — Red Hat corporate style compliance.
  - `verify-accessibility-compliance*` — WCAG / Section 508 validation.
  - `verify-technical-accuracy-*` — command and config correctness.
  - `verify-workshop-structure` — pedagogical structure validation.

When working on a topic that matches a skill, load it via the `skill` tool. When delegating, use the matching `@agent-name`.

## Local Development Commands

### Test Antora Site Build

```bash
cd showroom
podman run --rm -ti -v ${PWD}/..:/antora \
  registry.redhat.io/openshift4/ose-antora:latest \
  antora showroom/site.yml --to-dir showroom/www
```

### Validate Helm Charts

```bash
cd bootstrap/helm/tempo-stack
helm template . --values values.yaml | kubectl apply --dry-run=client -f -
```

## Debugging Deployment Issues

### ArgoCD Application Out of Sync

```bash
oc get application tempo-stack -n openshift-gitops -o yaml | yq '.status.conditions'
oc annotate application tempo-stack -n openshift-gitops argocd.argoproj.io/refresh=normal --overwrite
```

### Operator Not Installing (Common Issue)

```bash
oc get csv -n openshift-operators | grep tempo
oc get installplan -n openshift-operators
# Stuck? Delete and let ArgoCD recreate:
oc delete subscription tempo-product -n openshift-operators
```

### TempoStack/MonitoringStack Not Reconciling

1. Operator pod logs: `oc logs -n openshift-operators deployment/tempo-operator-controller`.
2. CR status conditions: `oc describe tempostack tempo -n openshift-tempo-operator | grep -A20 "Conditions:"`.
3. Common errors: schema validation, OBC secret field mismatch, NooBaa TLS issues.

## Files to Never Modify Manually

- `content/modules/ROOT/nav.adoc` — Updated by content skills only.
- Generated Antora assets in `www/` — Build output, not source.
- `.cache/antora/` — Antora build cache.

## Common Mistakes to Avoid

1. Hardcoding cluster URLs in AsciiDoc — use `{openshift_cluster_console_url}`.
2. Creating an `OperatorGroup` in `openshift-operators` — global namespace already has one.
3. Missing sync waves — CRs deploy before CRDs exist (add wave annotations).
4. Using the OBC secret directly for Tempo — Tempo requires reformatted secret fields.
5. Forgetting `targetRevision` in `ApplicationSet` — defaults to HEAD, specify `main`.

## Testing Checklist Before PR

- [ ] All Helm charts render without errors: `helm template . --values values.yaml`.
- [ ] Antora site builds: `podman run ... antora site.yml`.
- [ ] Nav structure updated if pages added/removed.
- [ ] AsciiDoc uses attribute placeholders, not hardcoded values.
- [ ] ArgoCD sync waves set on all resources.
- [ ] OpenShift 4.21+ compatibility verified (operator versions match).
