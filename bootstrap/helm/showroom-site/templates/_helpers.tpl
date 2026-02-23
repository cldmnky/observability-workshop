{{- /*
Sanitize namespace name for use in Kubernetes resource names.
Replaces dots and underscores with hyphens to ensure valid resource names.
Usage: {{ include "showroom-site.sanitize-namespace" "my.namespace_name" }}
*/ -}}
{{- define "showroom-site.sanitize-namespace" -}}
{{- . | replace "." "-" | replace "_" "-" -}}
{{- end -}}
